/**
 *
 * (c) Copyright Ascensio System SIA 2023
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

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
	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/onlyoffice"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/gateway/web/command"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared/request"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared/response"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/csrf"
	"github.com/gorilla/sessions"
	"go-micro.dev/v4/client"
	"golang.org/x/oauth2"
	"golang.org/x/sync/semaphore"
	"google.golang.org/api/drive/v2"
	"google.golang.org/api/option"
)

type FileController struct {
	jwtManager crypto.JwtManager
	fileUtil   onlyoffice.OnlyofficeFileUtility
	store      *sessions.CookieStore
	server     *config.ServerConfig
	onlyoffice *shared.OnlyofficeConfig
	credetials *oauth2.Config
	client     client.Client
	logger     log.Logger
}

func NewFileController(
	jwtManager crypto.JwtManager, fileUtil onlyoffice.OnlyofficeFileUtility,
	server *config.ServerConfig, onlyoffice *shared.OnlyofficeConfig,
	credentials *oauth2.Config, client client.Client, logger log.Logger,
) FileController {
	return FileController{
		fileUtil:   fileUtil,
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

		gclient := c.credetials.Client(ctx, &oauth2.Token{
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

func (c FileController) BuildConvertPage() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "text/html")
		qstate := r.URL.Query().Get("state")
		state := request.DriveState{
			UserAgent: r.UserAgent(),
		}

		if err := json.Unmarshal([]byte(qstate), &state); err != nil {
			errorPage.Execute(rw, nil)
			return
		}

		session, _ := c.store.Get(r, state.UserID)
		val, ok := session.Values["token"].(string)
		if !ok {
			c.logger.Debugf("could not cast a session jwt")
			session.Options.MaxAge = -1
			session.Save(r, rw)
			http.Redirect(rw, r.WithContext(r.Context()), c.credetials.AuthCodeURL(
				"state-token", oauth2.AccessTypeOffline, oauth2.ApprovalForce,
			), http.StatusSeeOther)
			return
		}

		var token jwt.RegisteredClaims
		if err := c.jwtManager.Verify(c.credetials.ClientSecret, val, &token); err != nil {
			c.logger.Warnf("could not verify a jwt: %s", err.Error())
			http.Redirect(rw, r.WithContext(r.Context()), c.credetials.AuthCodeURL(
				"state-token", oauth2.AccessTypeOffline, oauth2.ApprovalForce,
			), http.StatusSeeOther)
			return
		}

		c.logger.Debugf("jwt %s is valid", val)

		signature, err := c.jwtManager.Sign(c.credetials.ClientSecret, jwt.RegisteredClaims{
			ID:        state.UserID,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * 7 * time.Hour)),
		})

		if err != nil {
			errorPage.Execute(rw, nil)
			return
		}

		session.Values["token"] = signature
		session.Options.MaxAge = 60 * 60 * 23 * 7
		if err := session.Save(r, rw); err != nil {
			c.logger.Errorf("could not save a new session cookie: %s", err.Error())
			errorPage.Execute(rw, nil)
			return
		}

		var ures response.UserResponse
		if err := c.client.Call(r.Context(), c.client.NewRequest(
			fmt.Sprintf("%s:auth", c.server.Namespace), "UserSelectHandler.GetUser",
			fmt.Sprint(state.UserID),
		), &ures); err != nil {
			c.logger.Debugf("could not get user %s access info: %s", state.UserID, err.Error())
			errorPage.Execute(rw, nil)
			return
		}

		gclient := c.credetials.Client(r.Context(), &oauth2.Token{
			AccessToken:  ures.AccessToken,
			TokenType:    ures.TokenType,
			RefreshToken: ures.RefreshToken,
		})

		srv, err := drive.NewService(r.Context(), option.WithHTTPClient(gclient))
		if err != nil {
			c.logger.Errorf("Unable to retrieve drive service: %v", err)
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}

		file, err := srv.Files.Get(state.IDS[0]).Do()
		if err != nil {
			c.logger.Errorf("could not get file %s: %s", state.IDS[0], err.Error())
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}

		_, gdriveFile := shared.GdriveMimeOnlyofficeExtension[file.MimeType]
		if c.fileUtil.IsExtensionEditable(file.FileExtension) || c.fileUtil.IsExtensionViewOnly(file.FileExtension) || gdriveFile {
			http.Redirect(rw, r, fmt.Sprintf("/api/editor?state=%s", qstate), http.StatusMovedPermanently)
			return
		}

		convertPage.Execute(rw, map[string]interface{}{
			csrf.TemplateTag: csrf.TemplateField(r),
			"OOXML":          c.fileUtil.IsExtensionOOXMLConvertable(file.FileExtension),
			"LossEdit":       c.fileUtil.IsExtensionLossEditable(file.FileExtension),
		})
	}
}

func (c FileController) BuildConvertFile() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		var body request.ConvertRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			c.logger.Errorf("could not parse gdrive state: %s", err.Error())
			rw.WriteHeader(http.StatusBadRequest)
			return
		}

		session, err := c.store.Get(r, body.State.UserID)
		if err != nil {
			c.logger.Debugf("could not get session store: %s", err.Error())
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}

		val, ok := session.Values["token"].(string)
		if !ok {
			c.logger.Debugf("could not cast a session jwt")
			rw.WriteHeader(http.StatusForbidden)
			return
		}

		var token jwt.MapClaims
		if err := c.jwtManager.Verify(c.credetials.ClientSecret, val, &token); err != nil {
			c.logger.Debugf("could not verify a jwt: %s", err.Error())
			rw.WriteHeader(http.StatusForbidden)
			return
		}

		if token["jti"] != body.State.UserID {
			c.logger.Debugf("user with state id %s doesn't match token's id %s", body.State.UserID, token["jti"])
			rw.WriteHeader(http.StatusForbidden)
			return
		}

		switch body.Action {
		case "view":
			command.NewViewCommand().Execute(rw, r, &body.State)
			return
		case "edit":
			command.NewEditCommand().Execute(rw, r, &body.State)
			return
		case "create":
			command.NewConvertCommand(
				c.client, c.credetials, c.fileUtil, c.jwtManager, c.server, c.onlyoffice, c.logger,
			).Execute(rw, r, &body.State)
			return
		default:
			command.NewViewCommand().Execute(rw, r, &body.State)
		}
	}
}
