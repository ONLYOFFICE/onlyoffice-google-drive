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
	"fmt"
	"net/http"
	"net/url"

	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/gateway/web/embeddable"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared/request"
	"github.com/ONLYOFFICE/onlyoffice-integration-adapters/log"
	"github.com/ONLYOFFICE/onlyoffice-integration-adapters/onlyoffice"
	"github.com/gorilla/csrf"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"google.golang.org/api/drive/v2"
	goauth "google.golang.org/api/oauth2/v2"
)

type ConvertController struct {
	fileUtil      onlyoffice.OnlyofficeFileUtility
	apiController APIController
	logger        log.Logger
}

func NewConvertController(
	fileUtil onlyoffice.OnlyofficeFileUtility,
	apiController APIController,
	logger log.Logger,
) ConvertController {
	return ConvertController{
		fileUtil:      fileUtil,
		apiController: apiController,
		logger:        logger,
	}
}

func (c ConvertController) BuildConvertFile() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		var state request.DriveState
		if err := json.NewDecoder(r.Body).Decode(&state); err != nil {
			c.logger.Errorf("could not parse gdrive state: %s", err.Error())
			rw.WriteHeader(http.StatusBadRequest)
			return
		}

		switch state.Action {
		case "edit":
			state.ForceEdit = true
			http.Redirect(
				rw, r,
				fmt.Sprintf(
					"/editor?state=%s",
					url.QueryEscape(string(state.ToBytes())),
				),
				http.StatusMovedPermanently,
			)
			return
		case "view":
			http.Redirect(
				rw, r,
				fmt.Sprintf(
					"/editor?state=%s",
					url.QueryEscape(string(state.ToBytes())),
				),
				http.StatusMovedPermanently,
			)
		case "create":
			nstate, err := c.apiController.convertFile(r.Context(), &state)
			if err != nil {
				rw.WriteHeader(http.StatusInternalServerError)
				return
			}
			http.Redirect(
				rw, r,
				fmt.Sprintf(
					"/editor?state=%s",
					url.QueryEscape(string(nstate.ToBytes())),
				),
				http.StatusMovedPermanently,
			)
			return
		default:
			rw.WriteHeader(http.StatusBadGateway)
			return
		}
	}
}

func (c ConvertController) BuildConvertPage() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		qstate := r.URL.Query().Get("state")
		usr, uok := r.Context().Value("info").(goauth.Userinfo)
		file, fok := r.Context().Value("file").(drive.File)
		var state request.DriveState
		if err := json.Unmarshal([]byte(qstate), &state); err != nil || !uok || !fok {
			c.logger.Debug("could not unmarshal state or get session data")
			http.Redirect(rw, r, "https://drive.google.com", http.StatusMovedPermanently)
			return
		}

		loc := i18n.NewLocalizer(embeddable.Bundle, usr.Locale)

		if !file.Capabilities.CanCopy {
			embeddable.ErrorPage.ExecuteTemplate(rw, "error", map[string]interface{}{
				"errorMain": loc.MustLocalize(&i18n.LocalizeConfig{
					MessageID: "errorPermissionsMain",
				}),
				"errorSubtext": loc.MustLocalize(&i18n.LocalizeConfig{
					MessageID: "errorPermissionsSubtext",
				}),
				"reloadButton": loc.MustLocalize(&i18n.LocalizeConfig{
					MessageID: "reloadButton",
				}),
			})
			return
		}

		c.logger.Debugf("successfully found file with id %s", file.Id)
		_, gdriveFile := shared.GdriveMimeOnlyofficeExtension[file.MimeType]
		if !file.Capabilities.CanEdit || c.fileUtil.IsExtensionEditable(file.FileExtension) || c.fileUtil.IsExtensionViewOnly(file.FileExtension) || gdriveFile {
			http.Redirect(rw, r, fmt.Sprintf("/editor?state=%s", qstate), http.StatusMovedPermanently)
			return
		}

		rw.Header().Set("Content-Type", "text/html")
		embeddable.ConvertPage.Execute(rw, map[string]interface{}{
			csrf.TemplateTag: csrf.TemplateField(r),
			"Locale":         usr.Locale,
			"Title":          file.OriginalFilename,
			"OOXML": file.FileExtension != "csv" && (c.fileUtil.
				IsExtensionOOXMLConvertable(file.FileExtension) || c.fileUtil.IsExtensionLossEditable(file.FileExtension)),
			"LossEdit": c.fileUtil.IsExtensionLossEditable(file.FileExtension),
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
