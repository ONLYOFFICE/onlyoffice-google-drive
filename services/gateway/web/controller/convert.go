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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
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
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/csrf"
	"github.com/gorilla/sessions"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"go-micro.dev/v4/client"
	"golang.org/x/oauth2"
	"google.golang.org/api/drive/v2"
	goauth "google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"
)

var (
	_ErrSessionTokenCasting = errors.New("could not cast a session token")
	_ErrUserIdMatching      = errors.New("token uid and state uid do not match")
)

type ConvertController struct {
	client     client.Client
	emitter    events.Emitter
	jwtManager crypto.JwtManager
	fileUtil   onlyoffice.OnlyofficeFileUtility
	store      *sessions.CookieStore
	server     *config.ServerConfig
	credetials *oauth2.Config
	logger     log.Logger
}

func NewConvertController(
	client client.Client, emitter events.Emitter,
	jwtManager crypto.JwtManager, fileUtil onlyoffice.OnlyofficeFileUtility,
	server *config.ServerConfig, credentials *oauth2.Config, logger log.Logger,
) ConvertController {
	return ConvertController{
		client:     client,
		emitter:    emitter,
		jwtManager: jwtManager,
		fileUtil:   fileUtil,
		store:      sessions.NewCookieStore([]byte(credentials.ClientSecret)),
		server:     server,
		credetials: credentials,
		logger:     logger,
	}
}

func (c ConvertController) validateToken(state request.DriveState, session *sessions.Session) error {
	val, ok := session.Values["token"].(string)
	if !ok {
		c.logger.Debugf("could not cast a session jwt")
		return _ErrSessionTokenCasting
	}

	var token jwt.MapClaims
	if err := c.jwtManager.Verify(c.credetials.ClientSecret, val, &token); err != nil {
		c.logger.Warnf("could not verify a jwt: %s", err.Error())
		return err
	}

	if token["jti"] != state.UserID {
		return _ErrUserIdMatching
	}

	return nil
}

func (c ConvertController) BuildConvertPage() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "text/html")
		qstate := r.URL.Query().Get("state")
		state := request.DriveState{UserAgent: r.UserAgent()}
		errMsg := map[string]interface{}{
			"errorMain":    "Sorry, the document cannot be opened",
			"errorSubtext": "Please try again",
			"reloadButton": "Reload",
		}

		if err := json.Unmarshal([]byte(qstate), &state); err != nil {
			embeddable.ErrorPage.Execute(rw, errMsg)
			return
		}

		session, _ := c.store.Get(r, state.UserID)
		if err := c.validateToken(state, session); err != nil {
			session.Options.MaxAge = -1
			session.Save(r, rw)
			http.Redirect(rw, r.WithContext(r.Context()), c.credetials.AuthCodeURL(
				"state-token", oauth2.AccessTypeOffline, oauth2.ApprovalForce,
			), http.StatusSeeOther)
			return
		}

		signature, err := c.jwtManager.Sign(c.credetials.ClientSecret, jwt.RegisteredClaims{
			ID:        state.UserID,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * 7 * time.Hour)),
		})

		if err != nil {
			embeddable.ErrorPage.Execute(rw, errMsg)
			return
		}

		var ures response.UserResponse
		if err := c.client.Call(r.Context(), c.client.NewRequest(
			fmt.Sprintf("%s:auth", c.server.Namespace), "UserSelectHandler.GetUser",
			fmt.Sprint(state.UserID),
		), &ures); err != nil {
			c.logger.Debugf("could not get user %s access info: %s", state.UserID, err.Error())
			embeddable.ErrorPage.Execute(rw, errMsg)
			return
		}

		gclient := c.credetials.Client(r.Context(), &oauth2.Token{
			AccessToken:  ures.AccessToken,
			TokenType:    ures.TokenType,
			RefreshToken: ures.RefreshToken,
		})

		var wg sync.WaitGroup
		wg.Add(2)
		errChan := make(chan error, 2)
		usrChan := make(chan *goauth.Userinfo, 1)
		fileChan := make(chan *drive.File, 1)

		go func() {
			defer wg.Done()
			userService, err := goauth.NewService(r.Context(), option.WithHTTPClient(gclient))
			if err != nil {
				c.logger.Errorf("could not initialize a new user service: %s", err.Error())
				errChan <- err
				return
			}

			usr, err := userService.Userinfo.Get().Do()
			if err != nil {
				c.logger.Errorf("could not get user info: %s", err.Error())
				errChan <- err
				return
			}

			session.Values["token"] = signature
			session.Values["locale"] = usr.Locale
			session.Options.MaxAge = 60 * 60 * 23 * 7
			if err := session.Save(r, rw); err != nil {
				c.logger.Errorf("could not save a new session cookie: %s", err.Error())
				errChan <- err
				return
			}

			c.logger.Debugf("successfully found user with id %s", usr.Id)
			usrChan <- usr
		}()

		go func() {
			defer wg.Done()
			srv, err := drive.NewService(r.Context(), option.WithHTTPClient(gclient))
			if err != nil {
				c.logger.Errorf("Unable to retrieve drive service: %v", err)
				errChan <- err
				return
			}

			file, err := srv.Files.Get(state.IDS[0]).Do()
			if err != nil {
				c.logger.Errorf("could not get file %s: %s", state.IDS[0], err.Error())
				errChan <- err
				return
			}

			c.logger.Debugf("successfully found file with id %s", file.Id)
			fileChan <- file
		}()

		wg.Wait()

		select {
		case <-errChan:
			embeddable.ErrorPage.Execute(rw, errMsg)
			return
		default:
		}

		usr := <-usrChan
		file := <-fileChan

		_, gdriveFile := shared.GdriveMimeOnlyofficeExtension[file.MimeType]
		if c.fileUtil.IsExtensionEditable(file.FileExtension) || c.fileUtil.IsExtensionViewOnly(file.FileExtension) || gdriveFile {
			http.Redirect(rw, r, fmt.Sprintf("/api/editor?state=%s", qstate), http.StatusMovedPermanently)
			return
		}

		loc := i18n.NewLocalizer(embeddable.Bundle, usr.Locale)
		embeddable.ConvertPage.Execute(rw, map[string]interface{}{
			csrf.TemplateTag: csrf.TemplateField(r),
			"OOXML":          c.fileUtil.IsExtensionOOXMLConvertable(file.FileExtension),
			"LossEdit":       c.fileUtil.IsExtensionLossEditable(file.FileExtension),
			"openOnlyoffice": loc.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "openOnlyoffice",
			}),
			"cannotOpen": loc.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "cannotOpen",
			}),
			"selectAction": loc.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "selectAction",
			}),
			"openView": loc.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "openView",
			}),
			"createOOXML": loc.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "createOOXML",
			}),
			"editCopy": loc.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "editCopy",
			}),
			"openEditing": loc.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "openEditing",
			}),
			"moreInfo": loc.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "moreInfo",
			}),
			"dataRestrictions": loc.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "dataRestrictions",
			}),
			"openButton": loc.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "openButton",
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

func (c ConvertController) BuildConvertFile() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		var body request.ConvertRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			c.logger.Errorf("could not parse gdrive state: %s", err.Error())
			rw.WriteHeader(http.StatusBadRequest)
			return
		}

		c.emitter.Fire(body.Action, map[string]any{
			"writer":  rw,
			"request": r,
			"state":   &body.State,
		})
	}
}
