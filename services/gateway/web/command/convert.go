package command

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"time"

	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/config"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/crypto"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/functional"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/log"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/onlyoffice"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared/request"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared/response"
	"github.com/golang-jwt/jwt/v5"
	"go-micro.dev/v4/client"
	"golang.org/x/oauth2"
	"google.golang.org/api/drive/v2"
	"google.golang.org/api/option"
)

type ConvertCommand struct {
	client      client.Client
	credentials *oauth2.Config
	fileUtil    onlyoffice.OnlyofficeFileUtility
	jwtManager  crypto.JwtManager
	server      *config.ServerConfig
	onlyoffice  *shared.OnlyofficeConfig
	logger      log.Logger
}

func NewConvertCommand(
	client client.Client, credentials *oauth2.Config, fileUtil onlyoffice.OnlyofficeFileUtility,
	jwtManager crypto.JwtManager, server *config.ServerConfig, onlyoffice *shared.OnlyofficeConfig,
	logger log.Logger,
) Command {
	return &ConvertCommand{
		client:      client,
		credentials: credentials,
		fileUtil:    fileUtil,
		jwtManager:  jwtManager,
		server:      server,
		onlyoffice:  onlyoffice,
		logger:      logger,
	}
}

type convertInputOutput struct {
	Ctx           context.Context
	Service       *drive.Service
	State         *request.DriveState
	Creq          *request.ConvertAPIRequest
	Cres          *response.ConvertResponse
	Ures          *response.UserResponse
	File          *drive.File
	DownloadToken string
}

func (c *ConvertCommand) getFile(input convertInputOutput) (convertInputOutput, error) {
	gclient := c.credentials.Client(input.Ctx, &oauth2.Token{
		AccessToken:  input.Ures.AccessToken,
		TokenType:    input.Ures.TokenType,
		RefreshToken: input.Ures.RefreshToken,
	})

	srv, err := drive.NewService(input.Ctx, option.WithHTTPClient(gclient))
	if err != nil {
		c.logger.Errorf("Unable to retrieve drive service: %v", err)
		return convertInputOutput{}, err
	}

	file, err := srv.Files.Get(input.State.IDS[0]).Do()
	if err != nil {
		c.logger.Errorf("could not get file %s: %s", input.State.IDS[0], err.Error())
		return convertInputOutput{}, err
	}

	input.File = file
	return convertInputOutput{
		Ctx:     input.Ctx,
		Service: srv,
		State:   input.State,
		Ures:    input.Ures,
		File:    file,
	}, nil
}

func (c *ConvertCommand) generateDownloadToken(input convertInputOutput) (convertInputOutput, error) {
	downloadToken := &request.DriveDownloadToken{
		UserID: input.State.UserID,
		FileID: input.State.IDS[0],
	}
	downloadToken.IssuedAt = jwt.NewNumericDate(time.Now())
	downloadToken.ExpiresAt = jwt.NewNumericDate(time.Now().Add(4 * time.Minute))
	tkn, err := c.jwtManager.Sign(c.credentials.ClientSecret, downloadToken)
	return convertInputOutput{
		Ctx:           input.Ctx,
		Service:       input.Service,
		State:         input.State,
		Ures:          input.Ures,
		File:          input.File,
		DownloadToken: tkn,
	}, err
}

func (c *ConvertCommand) sendConvertRequest(input convertInputOutput) (convertInputOutput, error) {
	var cresp response.ConvertResponse
	fType, err := c.fileUtil.GetFileType(input.File.FileExtension)
	if err != nil {
		c.logger.Debugf("could not get file type: %s", err.Error())
		return input, err
	}

	creq := request.ConvertAPIRequest{
		Async:      false,
		Filetype:   fType,
		Outputtype: "ooxml",
		URL: fmt.Sprintf(
			"%s/api/download?token=%s", c.onlyoffice.Onlyoffice.Builder.GatewayURL,
			input.DownloadToken,
		),
	}
	creq.IssuedAt = jwt.NewNumericDate(time.Now())
	creq.ExpiresAt = jwt.NewNumericDate(time.Now().Add(2 * time.Minute))
	ctok, err := c.jwtManager.Sign(
		c.onlyoffice.Onlyoffice.Builder.DocumentServerSecret,
		creq,
	)

	if err != nil {
		return input, err
	}

	creq.Token = ctok
	req, err := http.NewRequest(
		"POST",
		fmt.Sprintf("%s/ConvertService.ashx", c.onlyoffice.Onlyoffice.Builder.DocumentServerURL),
		bytes.NewBuffer(creq.ToJSON()),
	)

	if err != nil {
		c.logger.Debugf("could not build a conversion api request: %s", err.Error())
		return input, err
	}

	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.logger.Errorf("could not send a conversion api request: %s", err.Error())
		return input, err
	}

	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(&cresp); err != nil {
		c.logger.Errorf("could not decode convert response body: %s", err.Error())
		return input, err
	}

	return convertInputOutput{
		Ctx:           input.Ctx,
		Service:       input.Service,
		State:         input.State,
		Creq:          &creq,
		Cres:          &cresp,
		Ures:          input.Ures,
		File:          input.File,
		DownloadToken: input.DownloadToken,
	}, nil
}

func (c *ConvertCommand) uploadConvertedFile(input convertInputOutput) (convertInputOutput, error) {
	cfile, err := http.Get(input.Cres.FileURL)
	if err != nil {
		c.logger.Errorf("could not retreive a converted file: %s", err.Error())
		return input, err
	}

	defer cfile.Body.Close()
	now := time.Now().Format(time.RFC3339)
	filename := fmt.Sprintf("%s.%s", input.File.Title[:len(input.File.Title)-len(filepath.Ext(input.File.Title))], input.Cres.FileType)
	file, err := input.Service.Files.Insert(&drive.File{
		DriveId:                      input.File.DriveId,
		CreatedDate:                  now,
		ModifiedDate:                 now,
		ModifiedByMeDate:             now,
		Capabilities:                 input.File.Capabilities,
		ContentRestrictions:          input.File.ContentRestrictions,
		CopyRequiresWriterPermission: input.File.CopyRequiresWriterPermission,
		DefaultOpenWithLink:          input.File.DefaultOpenWithLink,
		Description:                  input.File.Description,
		FileExtension:                input.Cres.FileType,
		OriginalFilename:             filename,
		OwnedByMe:                    true,
		Title:                        filename,
		Parents:                      input.File.Parents,
		MimeType:                     mime.TypeByExtension(fmt.Sprintf(".%s", input.Cres.FileType)),
	}).Media(cfile.Body).Do()

	if err != nil {
		c.logger.Errorf("could not modify file %s: %s", input.State.IDS[0], err.Error())
		return input, err
	}

	return convertInputOutput{
		Ctx:     input.Ctx,
		Service: input.Service,
		State: &request.DriveState{
			IDS:       []string{file.Id},
			Action:    input.State.Action,
			UserID:    input.State.UserID,
			UserAgent: input.Service.UserAgent,
		},
		Creq:          input.Creq,
		Cres:          input.Cres,
		Ures:          input.Ures,
		File:          input.File,
		DownloadToken: input.DownloadToken,
	}, nil
}

func (c *ConvertCommand) Execute(rw http.ResponseWriter, r *http.Request, state *request.DriveState) {
	res, err := functional.NewPipe[convertInputOutput]().
		Next(func(input convertInputOutput) (convertInputOutput, error) {
			var ures response.UserResponse
			if err := c.client.Call(r.Context(), c.client.NewRequest(
				fmt.Sprintf("%s:auth", c.server.Namespace), "UserSelectHandler.GetUser",
				fmt.Sprint(state.UserID),
			), &ures); err != nil {
				c.logger.Errorf("could not get user %s access info: %s", state.UserID, err.Error())
				return input, err
			}

			return convertInputOutput{
				Ctx:   r.Context(),
				State: state,
				Ures:  &ures,
			}, nil
		}).
		Next(c.getFile).
		Next(c.generateDownloadToken).
		Next(c.sendConvertRequest).
		Next(c.uploadConvertedFile).
		Do()

	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	http.Redirect(
		rw, r,
		fmt.Sprintf("/api/editor?state=%s", url.QueryEscape(string(res.State.ToJSON()))),
		http.StatusMovedPermanently,
	)
}
