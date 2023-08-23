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
	"errors"
	"fmt"
	"net/http"

	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/gateway/web/embeddable"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared/request"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared/response"
	"github.com/ONLYOFFICE/onlyoffice-integration-adapters/config"
	"github.com/ONLYOFFICE/onlyoffice-integration-adapters/crypto"
	"github.com/ONLYOFFICE/onlyoffice-integration-adapters/log"
	"github.com/gorilla/sessions"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"go-micro.dev/v4/client"
	"golang.org/x/oauth2"
)

type EditorController struct {
	client      client.Client
	jwtManager  crypto.JwtManager
	store       *sessions.CookieStore
	server      *config.ServerConfig
	credentials *oauth2.Config
	logger      log.Logger
}

func NewEditorController(
	client client.Client,
	jwtManager crypto.JwtManager,
	server *config.ServerConfig,
	credentials *oauth2.Config,
	logger log.Logger,
) EditorController {
	return EditorController{
		client:      client,
		jwtManager:  jwtManager,
		store:       sessions.NewCookieStore([]byte(credentials.ClientSecret)),
		server:      server,
		credentials: credentials,
		logger:      logger,
	}
}

func (c EditorController) BuildEditorPage() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "text/html")
		query := r.URL.Query()
		qstate := query.Get("state")
		var state request.DriveState
		if err := json.Unmarshal([]byte(qstate), &state); err != nil {
			c.logger.Debug("state is empty")
			http.Redirect(rw, r, "https://drive.google.com/", http.StatusMovedPermanently)
			return
		}

		var resp response.ConfigResponse
		if err := c.client.Call(r.Context(),
			c.client.NewRequest(fmt.Sprintf("%s:builder", c.server.Namespace), "ConfigHandler.BuildConfig", state),
			&resp,
		); err != nil {
			c.logger.Errorf("could not build onlyoffice config: %s", err.Error())
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				rw.WriteHeader(http.StatusRequestTimeout)
			}

			microErr := response.MicroError{}
			if err := json.Unmarshal([]byte(err.Error()), &microErr); err != nil {
				rw.WriteHeader(http.StatusInternalServerError)
			}

			c.logger.Errorf("build config micro error: %s", microErr.Detail)
			embeddable.ErrorPage.Execute(rw, map[string]interface{}{
				"errorMain":    "Sorry, the document cannot be opened",
				"errorSubtext": "Please try again",
				"reloadButton": "Reload",
			})
			return
		}

		c.logger.Debug("successfully saved a new session cookie")

		loc := i18n.NewLocalizer(embeddable.Bundle, resp.EditorConfig.Lang)
		embeddable.EditorPage.Execute(rw, map[string]interface{}{
			"apijs":   fmt.Sprintf("%s/web-apps/apps/api/documents/api.js", resp.ServerURL),
			"config":  string(resp.ToJSON()),
			"docType": resp.DocumentType,
			"cancelButton": loc.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "cancelButton",
			}),
		})
	}
}
