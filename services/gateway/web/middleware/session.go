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

package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/gateway/web/embeddable"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared/request"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared/response"
	"github.com/ONLYOFFICE/onlyoffice-integration-adapters/config"
	"github.com/ONLYOFFICE/onlyoffice-integration-adapters/crypto"
	"github.com/ONLYOFFICE/onlyoffice-integration-adapters/log"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/sessions"
	"go-micro.dev/v4/client"
	"go-micro.dev/v4/logger"
	"golang.org/x/oauth2"
	"google.golang.org/api/drive/v2"
	goauth "google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"
)

var (
	ErrEmptyResponse = errors.New("got a nil response")

	CreateAction = "create"
)

type SessionMiddleware struct {
	client      client.Client
	jwtManager  crypto.JwtManager
	store       *sessions.CookieStore
	credentials *oauth2.Config
	server      *config.ServerConfig
	logger      log.Logger
}

func NewSessionMiddleware(
	client client.Client,
	jwtManager crypto.JwtManager,
	credentials *oauth2.Config,
	server *config.ServerConfig,
	logger log.Logger,
) SessionMiddleware {
	return SessionMiddleware{
		client:      client,
		jwtManager:  jwtManager,
		store:       sessions.NewCookieStore([]byte(credentials.ClientSecret)),
		credentials: credentials,
		server:      server,
		logger:      logger,
	}
}

func (c SessionMiddleware) getServices(ctx context.Context, user response.UserResponse) (*drive.Service, *goauth.Service, error) {
	client := c.credentials.Client(ctx, &oauth2.Token{
		AccessToken:  user.AccessToken,
		TokenType:    user.TokenType,
		RefreshToken: user.RefreshToken,
	})

	dsrv, err := drive.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, nil, err
	}

	asrv, err := goauth.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, nil, err
	}

	return dsrv, asrv, nil
}

func (m SessionMiddleware) renderError(rw http.ResponseWriter) {
	rw.Header().Set("Content-Type", "text/html")
	if err := embeddable.ErrorPage.ExecuteTemplate(rw, "error", map[string]interface{}{
		"errorMain":    "Sorry, the document cannot be opened",
		"errorSubtext": "Please try again",
		"reloadButton": "Reload",
	}); err != nil {
		m.logger.Errorf("could not execute an error template: %w", err)
	}
}

// TODO: Caching
func (m SessionMiddleware) Protect(next http.Handler) http.Handler {
	fn := func(rw http.ResponseWriter, r *http.Request) {
		state := request.DriveState{UserAgent: r.UserAgent()}
		if err := json.Unmarshal([]byte(r.URL.Query().Get("state")), &state); err != nil {
			logger.Debugf("could not unmarshal gdrive state: %s", err.Error())
			http.Redirect(rw, r, "https://drive.google.com", http.StatusMovedPermanently)
			return
		}

		session, err := m.store.Get(r, "onlyoffice-auth")
		if err != nil {
			m.logger.Errorf("could not get session for user %s: %s", state.UserID, err.Error())
			http.Redirect(rw, r.WithContext(r.Context()), m.credentials.AuthCodeURL(
				"state-token", oauth2.AccessTypeOffline, oauth2.ApprovalForce,
			), http.StatusSeeOther)
			return
		}

		val, ok := session.Values["token"].(string)
		if !ok {
			m.logger.Debug("could not cast token to string")
			session.Options.MaxAge = -1
			if err := session.Save(r, rw); err != nil {
				m.logger.Errorf("could not save a cookie session: %w", err)
			}
			http.Redirect(rw, r.WithContext(r.Context()), m.credentials.AuthCodeURL(
				"state-token", oauth2.AccessTypeOffline, oauth2.ApprovalForce,
			), http.StatusSeeOther)
			return
		}

		var token jwt.MapClaims
		if err := m.jwtManager.Verify(m.credentials.ClientSecret, val, &token); err != nil {
			m.logger.Debugf("could not verify session token: %s", err.Error())
			session.Options.MaxAge = -1
			if err := session.Save(r, rw); err != nil {
				m.logger.Errorf("could not save a cookie session: %w", err)
			}
			http.Redirect(rw, r.WithContext(r.Context()), m.credentials.AuthCodeURL(
				"state-token", oauth2.AccessTypeOffline, oauth2.ApprovalForce,
			), http.StatusSeeOther)
			return
		}

		if token["jti"] != state.UserID {
			m.logger.Debugf("user % doesn't match state user %s", token["jti"], state.UserID)
			session.Options.MaxAge = -1
			if err := session.Save(r, rw); err != nil {
				m.logger.Errorf("could not save a cookie session: %w", err)
			}
			http.Redirect(rw, r.WithContext(r.Context()), m.credentials.AuthCodeURL(
				"state-token", oauth2.AccessTypeOffline, oauth2.ApprovalForce,
			), http.StatusSeeOther)
			return
		}

		var ures response.UserResponse
		tctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		if err := m.client.Call(tctx, m.client.NewRequest(
			fmt.Sprintf("%s:auth", m.server.Namespace), "UserSelectHandler.GetUser",
			fmt.Sprint(state.UserID),
		), &ures); err != nil {
			http.Redirect(rw, r.WithContext(r.Context()), m.credentials.AuthCodeURL(
				"state-token", oauth2.AccessTypeOffline, oauth2.ApprovalForce,
			), http.StatusSeeOther)
			return
		}

		srv, asrv, err := m.getServices(r.Context(), ures)
		if err != nil {
			m.logger.Debugf("could not retreive a gdrive service for user %s. Reason: %s",
				state.UserID, err.Error())
			m.renderError(rw)
			return
		}

		var wg sync.WaitGroup
		wg.Add(2)
		errChan := make(chan error, 2)
		fileChan := make(chan drive.File, 1)
		userChan := make(chan goauth.Userinfo, 1)

		go func() {
			defer wg.Done()
			m.logger.Debugf("state has %d file ids", len(state.IDS))
			id := ""
			if len(state.IDS) > 0 {
				id = state.IDS[0]
			} else if len(state.ExportIDS) > 0 {
				id = state.ExportIDS[0]
			}

			if id != "" {
				file, err := srv.Files.Get(id).Do()
				if err != nil {
					m.logger.Errorf("could not get file %s: %s", id, err.Error())
					errChan <- err
					return
				}

				if file == nil {
					m.logger.Errorf("got a nil pointer to gdrive file")
					errChan <- ErrEmptyResponse
					return
				}

				m.logger.Debugf("found a file %s", file.Id)
				fileChan <- *file
				return
			}

			if state.Action == CreateAction {
				m.logger.Debugf("setting an empty file for create action")
				fileChan <- drive.File{}
				return
			}

			m.logger.Error("gdrive state has no file id")
			errChan <- ErrEmptyResponse
		}()

		go func() {
			defer wg.Done()
			uinfo, err := asrv.Userinfo.Get().Do()
			if err != nil {
				m.logger.Errorf("could not get user info: %s", err.Error())
				errChan <- err
				return
			}

			if uinfo == nil {
				m.logger.Errorf("got a nil pointer to gdrive user info")
				errChan <- ErrEmptyResponse
				return
			}

			m.logger.Debugf("got user info with id %s", uinfo.Id)
			userChan <- *uinfo
		}()

		wg.Wait()

		select {
		case <-errChan:
			m.renderError(rw)
			return
		case <-tctx.Done():
			m.renderError(rw)
			return
		default:
		}

		signature, _ := m.jwtManager.Sign(m.credentials.ClientSecret, jwt.RegisteredClaims{
			ID:        state.UserID,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * 7 * time.Hour)),
		})
		session.Values["token"] = signature
		if err := session.Save(r, rw); err != nil {
			m.logger.Errorf("could not save session token: %s", err.Error())
		}

		m.logger.Debugf("refreshed current session: %s", signature)

		// nolint:staticcheck
		next.ServeHTTP(rw, r.WithContext(
			context.WithValue(
				context.WithValue(
					context.WithValue(r.Context(),
						"user", ures,
					), "file", <-fileChan),
				"info", <-userChan,
			)))
	}

	return http.HandlerFunc(fn)
}
