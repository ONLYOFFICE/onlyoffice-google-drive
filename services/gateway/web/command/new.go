package command

import (
	"errors"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"time"

	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/config"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/crypto"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/events"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/log"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/onlyoffice"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/gateway/web/embeddable"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared/request"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared/response"
	"go-micro.dev/v4/client"
	"golang.org/x/oauth2"
	"golang.org/x/sync/semaphore"
	"google.golang.org/api/drive/v2"
	"google.golang.org/api/option"
)

type newCommand struct {
	client      client.Client
	credentials *oauth2.Config
	fileUtil    onlyoffice.OnlyofficeFileUtility
	jwtManager  crypto.JwtManager
	server      *config.ServerConfig
	onlyoffice  *shared.OnlyofficeConfig
	sem         *semaphore.Weighted
	logger      log.Logger
}

func NewCommand(
	client client.Client, credentials *oauth2.Config, fileUtil onlyoffice.OnlyofficeFileUtility,
	jwtManager crypto.JwtManager, server *config.ServerConfig, onlyoffice *shared.OnlyofficeConfig,
	emitter events.Emitter, logger log.Logger,
) newCommand {
	sem := semaphore.NewWeighted(int64(onlyoffice.Onlyoffice.Builder.AllowedDownloads))
	c := newCommand{
		client:      client,
		credentials: credentials,
		fileUtil:    fileUtil,
		jwtManager:  jwtManager,
		server:      server,
		onlyoffice:  onlyoffice,
		sem:         sem,
		logger:      logger,
	}
	emitter.On("new", c)
	return c
}

func (c newCommand) Handle(e events.Event) error {
	rw := e.Get("writer").(http.ResponseWriter)
	r := e.Get("request").(*http.Request)
	payload := e.Get("payload").(*request.CreateRequest)
	if rw == nil || r == nil || payload == nil {
		return nil
	}

	if ok := c.sem.TryAcquire(1); !ok {
		rw.WriteHeader(http.StatusTooManyRequests)
		return errors.New("could not acquire semaphore")
	}

	defer c.sem.Release(1)

	var ures response.UserResponse
	if err := c.client.Call(r.Context(), c.client.NewRequest(
		fmt.Sprintf("%s:auth", c.server.Namespace), "UserSelectHandler.GetUser",
		fmt.Sprint(payload.State.UserID),
	), &ures); err != nil {
		c.logger.Errorf("could not get user %s access info to create a new file: %s", payload.State.UserID, err.Error())
		return err
	}

	gclient := c.credentials.Client(r.Context(), &oauth2.Token{
		AccessToken:  ures.AccessToken,
		TokenType:    ures.TokenType,
		RefreshToken: ures.RefreshToken,
	})

	srv, err := drive.NewService(r.Context(), option.WithHTTPClient(gclient))
	if err != nil {
		c.logger.Errorf("unable to retrieve drive service: %v", err)
		return err
	}

	file, err := embeddable.OfficeFiles.Open(fmt.Sprintf("files/en-US/new.%s", payload.Type))
	if err != nil {
		c.logger.Errorf("could not open a new file: %s", err.Error())
		return err
	}

	newFile, err := srv.Files.Insert(&drive.File{
		CreatedDate:      time.Now().Format(time.RFC3339),
		FileExtension:    payload.Type,
		MimeType:         mime.TypeByExtension(fmt.Sprintf(".%s", payload.Type)),
		Title:            fmt.Sprintf("New Document.%s", payload.Type),
		OriginalFilename: fmt.Sprintf("New Document.%s", payload.Type),
		Parents: []*drive.ParentReference{{
			Id: payload.State.FolderID,
		}},
	}).Media(file).Do()

	if err != nil {
		c.logger.Errorf("could not create a new file: %s", err.Error())
		return err
	}

	payload.State.IDS = []string{newFile.Id}
	http.Redirect(
		rw, r,
		fmt.Sprintf("/api/editor?state=%s", url.QueryEscape(string(payload.State.ToJSON()))),
		http.StatusMovedPermanently,
	)

	return nil
}
