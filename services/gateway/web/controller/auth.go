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
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/config"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/crypto"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/log"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared/response"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/sessions"
	"go-micro.dev/v4/client"
	"golang.org/x/oauth2"
	"golang.org/x/sync/singleflight"
	"google.golang.org/api/drive/v3"
	goauth "google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"
)

var group singleflight.Group

type AuthController struct {
	client     client.Client
	jwtManager crypto.JwtManager
	store      *sessions.CookieStore
	config     *config.ServerConfig
	oauth      *oauth2.Config
	logger     log.Logger
}

func NewAuthController(
	client client.Client,
	jwtManager crypto.JwtManager,
	config *config.ServerConfig,
	oauth *oauth2.Config,
	logger log.Logger,
) AuthController {
	return AuthController{
		client:     client,
		jwtManager: jwtManager,
		store:      sessions.NewCookieStore([]byte(oauth.ClientSecret)),
		config:     config,
		oauth:      oauth,
		logger:     logger,
	}
}

func (c AuthController) BuildGetAuth() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		code := strings.TrimSpace(r.URL.Query().Get("code"))
		if code == "" {
			rw.WriteHeader(http.StatusBadRequest)
			c.logger.Debug("empty auth code parameter")
			return
		}

		group.Do(code, func() (interface{}, error) {
			token, err := c.oauth.Exchange(r.Context(), code)
			if err != nil {
				c.logger.Errorf("could not get gdrive access token: %s", err.Error())
				return nil, err
			}

			client := c.oauth.Client(r.Context(), token)

			uservice, err := goauth.NewService(r.Context(), option.WithHTTPClient(client))
			if err != nil {
				c.logger.Errorf("could not create a new goauth service: %s", err.Error())
				return nil, err
			}

			uinfo, err := uservice.Userinfo.Get().Do()
			if err != nil {
				c.logger.Errorf("could not get user info: %s", err.Error())
				return nil, err
			}

			var resp interface{}
			if err := c.client.Call(r.Context(), c.client.NewRequest(fmt.Sprintf("%s:auth", c.config.Namespace), "UserInsertHandler.InsertUser", response.UserResponse{
				ID:           uinfo.Id,
				AccessToken:  token.AccessToken,
				RefreshToken: token.RefreshToken,
				TokenType:    token.TokenType,
				Scope:        strings.Join([]string{drive.DriveMetadataReadonlyScope, drive.DriveFileScope, drive.DriveReadonlyScope, shared.DriveInstall}, " "),
				Expiry:       token.Expiry.UTC().Format(time.RFC3339),
			}), &resp); err != nil {
				c.logger.Errorf("could not insert a new user: %s", err.Error())
				return nil, err
			}

			signature, err := c.jwtManager.Sign(c.oauth.ClientSecret, jwt.RegisteredClaims{
				ID:        uinfo.Id,
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
			})

			if err != nil {
				c.logger.Errorf("could not issue a new jwt: %s", err.Error())
				return nil, err
			}

			session, _ := c.store.Get(r, uinfo.Id)
			session.Values["token"] = signature
			session.Values["locale"] = uinfo.Locale
			session.Options.MaxAge = 60 * 60 * 23 * 7
			if err := session.Save(r, rw); err != nil {
				c.logger.Errorf("could not save a new session cookie: %s", err.Error())
				return nil, err
			}

			return nil, nil
		})

		http.Redirect(rw, r, "https://drive.google.com/", http.StatusMovedPermanently)
	}
}
