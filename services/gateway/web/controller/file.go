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
	"net/http"

	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/gateway/web/embeddable"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared/request"
	"github.com/ONLYOFFICE/onlyoffice-integration-adapters/config"
	"github.com/ONLYOFFICE/onlyoffice-integration-adapters/log"
	"github.com/gorilla/csrf"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/sync/semaphore"
	goauth "google.golang.org/api/oauth2/v2"
)

type FileController struct {
	sem    *semaphore.Weighted
	logger log.Logger
}

func NewFileController(
	server *config.ServerConfig,
	onlyoffice *shared.OnlyofficeConfig,
	logger log.Logger,
) FileController {
	return FileController{
		sem:    semaphore.NewWeighted(int64(onlyoffice.Onlyoffice.Builder.AllowedDownloads)),
		logger: logger,
	}
}

func (c FileController) BuildCreateFilePage() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "text/html")
		qstate := r.URL.Query().Get("state")
		user, uok := r.Context().Value("info").(goauth.Userinfo)
		state := request.DriveState{UserAgent: r.UserAgent()}
		if err := json.Unmarshal([]byte(qstate), &state); err != nil || !uok {
			http.Redirect(rw, r, "https://drive.google.com", http.StatusMovedPermanently)
			return
		}

		loc := i18n.NewLocalizer(embeddable.Bundle, user.Locale)
		embeddable.CreationPage.Execute(rw, map[string]interface{}{
			csrf.TemplateTag: csrf.TemplateField(r),
			"Locale":         user.Locale,
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
