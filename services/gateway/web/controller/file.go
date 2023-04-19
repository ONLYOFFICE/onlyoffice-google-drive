package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/config"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/crypto"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/log"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared/request"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared/response"
	"github.com/golang-jwt/jwt"
	"github.com/gorilla/sessions"
	"go-micro.dev/v4/client"
	"golang.org/x/oauth2"
	"golang.org/x/sync/semaphore"
	"google.golang.org/api/drive/v2"
	"google.golang.org/api/option"
)

type FileController struct {
	jwtManager crypto.JwtManager
	store      *sessions.CookieStore
	server     *config.ServerConfig
	onlyoffice *shared.OnlyofficeConfig
	credetials *oauth2.Config
	client     client.Client
	logger     log.Logger
}

func NewFileController(
	jwtManager crypto.JwtManager, server *config.ServerConfig, onlyoffice *shared.OnlyofficeConfig,
	credentials *oauth2.Config, client client.Client, logger log.Logger,
) FileController {
	return FileController{
		server:     server,
		store:      sessions.NewCookieStore([]byte(credentials.ClientSecret)),
		jwtManager: jwtManager,
		onlyoffice: onlyoffice,
		credetials: credentials,
		client:     client,
		logger:     logger,
	}
}

func (c FileController) BuildDownloadFile() http.HandlerFunc {
	sem := semaphore.NewWeighted(int64(c.onlyoffice.Onlyoffice.Builder.AllowedDownloads))
	return func(rw http.ResponseWriter, r *http.Request) {
		var dtoken request.DriveDownloadToken
		token := strings.TrimSpace(r.URL.Query().Get("token"))
		if token == "" {
			c.logger.Errorf("unauthorized access to an api endpoint")
			rw.WriteHeader(http.StatusUnauthorized)
			return
		}

		if ok := sem.TryAcquire(1); !ok {
			c.logger.Warn("too many download requests")
			rw.WriteHeader(http.StatusTooManyRequests)
			return
		}

		defer sem.Release(1)

		var wg sync.WaitGroup
		wg.Add(2)
		errChan := make(chan error, 2)

		go func() {
			defer wg.Done()
			if err := c.jwtManager.Verify(c.credetials.ClientSecret, token, &dtoken); err != nil {
				c.logger.Errorf("could not verify gdrive token: %s", err.Error())
				errChan <- err
				return
			}
		}()

		go func() {
			defer wg.Done()
			var tkn interface{}
			if err := c.jwtManager.Verify(
				c.onlyoffice.Onlyoffice.Builder.DocumentServerSecret,
				strings.ReplaceAll(
					r.Header.Get(c.onlyoffice.Onlyoffice.Builder.DocumentServerHeader), "Bearer ", "",
				),
				&tkn,
			); err != nil {
				c.logger.Errorf("could not verify docs header: %s", err.Error())
				errChan <- err
				return
			}
		}()

		wg.Wait()

		select {
		case err := <-errChan:
			c.logger.Errorf(err.Error())
			rw.WriteHeader(http.StatusForbidden)
			return
		default:
		}

		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()

		var ures response.UserResponse
		if err := c.client.Call(ctx, c.client.NewRequest(
			fmt.Sprintf("%s:auth", c.server.Namespace),
			"UserSelectHandler.GetUser", dtoken.UserID,
		), &ures); err != nil {
			c.logger.Debugf("could not get user %s access info: %s", dtoken.UserID, err.Error())
			rw.WriteHeader(http.StatusForbidden)
			return
		}

		gclient := c.credetials.Client(r.Context(), &oauth2.Token{
			AccessToken:  ures.AccessToken,
			TokenType:    ures.TokenType,
			RefreshToken: ures.RefreshToken,
		})

		srv, err := drive.NewService(ctx, option.WithHTTPClient(gclient))
		if err != nil {
			c.logger.Errorf("Unable to retrieve drive service: %v", err)
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}

		file, err := srv.Files.Get(dtoken.FileID).Do()
		if err != nil {
			c.logger.Errorf("could not get file %s: %s", dtoken.FileID, err.Error())
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}

		if mime, ok := shared.GdriveMimeOnlyofficeMime[file.MimeType]; ok {
			resp, err := srv.Files.Export(dtoken.FileID, mime).Download()
			if err != nil {
				c.logger.Errorf("Could not download file %s: %s", dtoken.FileID, err.Error())
				rw.WriteHeader(http.StatusInternalServerError)
				return
			}

			defer resp.Body.Close()
			io.Copy(rw, resp.Body)
		} else {
			resp, err := srv.Files.Get(dtoken.FileID).Download()
			if err != nil {
				c.logger.Errorf("Could not download file %s: %s", dtoken.FileID, err.Error())
				rw.WriteHeader(http.StatusInternalServerError)
				return
			}

			defer resp.Body.Close()
			io.Copy(rw, resp.Body)
		}
	}
}

func (c FileController) BuildConvertFile() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		qstate := r.URL.Query().Get("state")
		state := request.DriveState{
			UserAgent: r.UserAgent(),
		}

		if err := json.Unmarshal([]byte(qstate), &state); err != nil {
			c.logger.Errorf("could not parse gdrive state: %s", err.Error())
			rw.WriteHeader(http.StatusBadRequest)
			return
		}

		var body request.ConvertJobMessage
		err := json.NewDecoder(r.Body).Decode(&body)
		if err != nil {
			c.logger.Errorf("could not convert body to convert job message: %s", err.Error())
			rw.WriteHeader(http.StatusBadRequest)
			return
		}

		session, _ := c.store.Get(r, state.UserID)
		val, ok := session.Values["token"].(string)
		if !ok {
			c.logger.Debugf("could not cast a session jwt")
			rw.WriteHeader(http.StatusForbidden)
			return
		}

		var token jwt.StandardClaims
		if err := c.jwtManager.Verify(c.credetials.ClientSecret, val, &token); err != nil {
			c.logger.Warnf("could not verify a jwt: %s", err.Error())
			rw.WriteHeader(http.StatusForbidden)
			return
		}

		body.FileID = state.IDS[0]
		body.UserID = state.UserID
		var res interface{}
		if err := c.client.Call(r.Context(), c.client.NewRequest(
			fmt.Sprintf("%s:converter", c.server.Namespace),
			"ConvertHandler.Convert", body,
		), &res); err != nil {
			c.logger.Debugf("could send a conversion request: %s", err.Error())
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}

		rw.WriteHeader(http.StatusOK)
	}
}
