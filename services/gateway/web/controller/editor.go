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
	"github.com/ONLYOFFICE/onlyoffice-integration-adapters/log"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"go-micro.dev/v4/client"
	"google.golang.org/api/drive/v2"
	goauth "google.golang.org/api/oauth2/v2"
)

type EditorController struct {
	client client.Client
	server *config.ServerConfig
	logger log.Logger
}

func NewEditorController(
	client client.Client,
	server *config.ServerConfig,
	logger log.Logger,
) EditorController {
	return EditorController{
		client: client,
		server: server,
		logger: logger,
	}
}

func (c EditorController) BuildEditorPage() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "text/html")
		query := r.URL.Query()
		qstate := query.Get("state")
		usr, uok := r.Context().Value("info").(goauth.Userinfo)
		file, fok := r.Context().Value("file").(drive.File)
		var state request.DriveState
		if err := json.Unmarshal([]byte(qstate), &state); err != nil || !uok || !fok {
			c.logger.Debug("state is empty")
			http.Redirect(rw, r, "https://drive.google.com/", http.StatusMovedPermanently)
			return
		}

		loc := i18n.NewLocalizer(embeddable.Bundle, usr.Locale)
		var resp response.ConfigResponse
		if err := c.client.Call(r.Context(),
			c.client.NewRequest(
				fmt.Sprintf("%s:builder", c.server.Namespace), "ConfigHandler.BuildConfig",
				request.ConfigRequest{
					UserInfo:  usr,
					FileInfo:  file,
					UserAgent: state.UserAgent,
					ForceEdit: state.ForceEdit,
				}),
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
				"Locale": usr.Locale,
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
			return
		}

		c.logger.Debug("successfully saved a new session cookie")

		embeddable.EditorPage.Execute(rw, map[string]interface{}{
			"Locale":  usr.Locale,
			"Title":   file.OriginalFilename,
			"apijs":   fmt.Sprintf("%s/web-apps/apps/api/documents/api.js", resp.ServerURL),
			"config":  string(resp.ToBytes()),
			"docType": resp.DocumentType,
			"cancelButton": loc.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "cancelButton",
			}),
		})
	}
}
