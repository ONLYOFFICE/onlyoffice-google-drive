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
	"mime"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/gateway/web/embeddable"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared/request"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared/response"
	"github.com/ONLYOFFICE/onlyoffice-integration-adapters/config"
	"github.com/ONLYOFFICE/onlyoffice-integration-adapters/crypto"
	"github.com/ONLYOFFICE/onlyoffice-integration-adapters/log"
	"github.com/gorilla/csrf"
	"github.com/gorilla/sessions"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"go-micro.dev/v4/client"
	"golang.org/x/oauth2"
	"golang.org/x/sync/semaphore"
	"google.golang.org/api/drive/v2"
	"google.golang.org/api/option"
)

type FileController struct {
	client      client.Client
	jwtManager  crypto.JwtManager
	store       *sessions.CookieStore
	server      *config.ServerConfig
	onlyoffice  *shared.OnlyofficeConfig
	credentials *oauth2.Config
	sem         *semaphore.Weighted
	logger      log.Logger
}

func NewFileController(
	client client.Client, jwtManager crypto.JwtManager,
	server *config.ServerConfig, onlyoffice *shared.OnlyofficeConfig,
	credentials *oauth2.Config, logger log.Logger,
) FileController {
	return FileController{
		client:      client,
		jwtManager:  jwtManager,
		store:       sessions.NewCookieStore([]byte(credentials.ClientSecret)),
		server:      server,
		onlyoffice:  onlyoffice,
		credentials: credentials,
		sem:         semaphore.NewWeighted(int64(onlyoffice.Onlyoffice.Builder.AllowedDownloads)),
		logger:      logger,
	}
}

func (c FileController) BuildCreateFilePage() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "text/html")
		qstate := r.URL.Query().Get("state")
		state := request.DriveState{UserAgent: r.UserAgent()}
		tctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		if err := json.Unmarshal([]byte(qstate), &state); err != nil {
			http.Redirect(rw, r, "https://drive.google.com", http.StatusMovedPermanently)
			return
		}

		session, err := c.store.Get(r, "onlyoffice-auth")
		if err != nil {
			c.logger.Debugf("could not get auth session: %s", err.Error())
			http.Redirect(rw, r.WithContext(tctx), c.credentials.AuthCodeURL(
				"state-token", oauth2.AccessTypeOffline, oauth2.ApprovalForce,
			), http.StatusSeeOther)
			return
		}

		locale, ok := session.Values["locale"].(string)
		if !ok {
			c.logger.Debug("could not extract locale")
			locale = "en"
		}

		loc := i18n.NewLocalizer(embeddable.Bundle, locale)
		embeddable.CreationPage.Execute(rw, map[string]interface{}{
			csrf.TemplateTag: csrf.TemplateField(r),
			"createFilePlaceholder": loc.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "createFilePlaceholder",
			}),
			"createFileInput": loc.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "createFileInput",
			}),
			"createFileTitle": loc.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "createFileTitle",
			}),
			"createDocx": loc.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "createDocx",
			}),
			"createPptx": loc.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "createPptx",
			}),
			"createXlsx": loc.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "createXlsx",
			}),
			"createButton": loc.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "createButton",
			}),
			"cancelButton": loc.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "cancelButton",
			}),
			"errorMain": loc.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "errorMain",
			}),
			"errorSubtext": loc.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "errorSubtext",
			}),
			"reloadButton": loc.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "reloadButton",
			}),
		})
	}
}

func (c FileController) BuildCreateFile() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		uctx, cancel := context.WithTimeout(r.Context(), time.Duration(c.onlyoffice.Onlyoffice.Callback.UploadTimeout)*time.Second)
		defer cancel()
		var body request.DriveState
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			c.logger.Errorf("could not parse gdrive state: %s", err.Error())
			rw.WriteHeader(http.StatusBadRequest)
			return
		}

		if body.Filename == "" {
			body.Filename = "New Document"
		}

		if body.Action == "" {
			body.Action = "docx"
		}

		if ok := c.sem.TryAcquire(1); !ok {
			rw.WriteHeader(http.StatusTooManyRequests)
			return
		}

		defer c.sem.Release(1)

		var ures response.UserResponse
		if err := c.client.Call(uctx, c.client.NewRequest(
			fmt.Sprintf("%s:auth", c.server.Namespace), "UserSelectHandler.GetUser",
			fmt.Sprint(body.UserID),
		), &ures); err != nil {
			c.logger.Errorf("could not get user %s access info to create a new file: %s", body.UserID, err.Error())
			rw.WriteHeader(http.StatusBadRequest)
			return
		}

		gclient := c.credentials.Client(uctx, &oauth2.Token{
			AccessToken:  ures.AccessToken,
			TokenType:    ures.TokenType,
			RefreshToken: ures.RefreshToken,
		})

		srv, err := drive.NewService(uctx, option.WithHTTPClient(gclient))
		if err != nil {
			c.logger.Errorf("unable to retrieve drive service: %v", err)
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}

		file, err := embeddable.OfficeFiles.Open(fmt.Sprintf("files/en-US/new.%s", body.Action))
		if err != nil {
			c.logger.Errorf("could not open a new file: %s", err.Error())
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}

		newFile, err := srv.Files.Insert(&drive.File{
			CreatedDate:      time.Now().Format(time.RFC3339),
			FileExtension:    body.Action,
			MimeType:         mime.TypeByExtension(fmt.Sprintf(".%s", body.Action)),
			Title:            fmt.Sprintf("%s.%s", body.Filename, body.Action),
			OriginalFilename: fmt.Sprintf("%s.%s", body.Filename, body.Action),
			Parents: []*drive.ParentReference{{
				Id: body.FolderID,
			}},
		}).Context(uctx).Media(file).Do()

		if err != nil {
			c.logger.Errorf("could not create a new file: %s", err.Error())
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}

		body.IDS = []string{newFile.Id}
		http.Redirect(
			rw, r,
			fmt.Sprintf("/editor?state=%s", url.QueryEscape(string(body.ToJSON()))),
			http.StatusMovedPermanently,
		)
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
			if err := c.jwtManager.Verify(c.credentials.ClientSecret, token, &dtoken); err != nil {
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

		ctx, cancel := context.WithTimeout(r.Context(), 1*time.Minute)
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

		gclient := c.credentials.Client(ctx, &oauth2.Token{
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
