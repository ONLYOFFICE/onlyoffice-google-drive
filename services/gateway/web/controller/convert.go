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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"time"

	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/config"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/crypto"
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
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"golang.org/x/oauth2"
	"google.golang.org/api/drive/v2"
	"google.golang.org/api/option"
)

var (
	_ErrSessionTokenCasting = errors.New("could not cast a session token")
	_ErrUserIdMatching      = errors.New("token uid and state uid do not match")
)

type ConvertController struct {
	client      client.Client
	jwtManager  crypto.JwtManager
	fileUtil    onlyoffice.OnlyofficeFileUtility
	store       *sessions.CookieStore
	server      *config.ServerConfig
	credentials *oauth2.Config
	onlyoffice  *shared.OnlyofficeConfig
	logger      log.Logger
}

func NewConvertController(
	client client.Client, jwtManager crypto.JwtManager,
	fileUtil onlyoffice.OnlyofficeFileUtility, onlyoffice *shared.OnlyofficeConfig,
	server *config.ServerConfig, credentials *oauth2.Config, logger log.Logger,
) ConvertController {
	return ConvertController{
		client:      client,
		jwtManager:  jwtManager,
		fileUtil:    fileUtil,
		store:       sessions.NewCookieStore([]byte(credentials.ClientSecret)),
		server:      server,
		credentials: credentials,
		onlyoffice:  onlyoffice,
		logger:      logger,
	}
}

func (c ConvertController) BuildConvertPage() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "text/html")
		qstate := r.URL.Query().Get("state")
		tctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		errMsg := map[string]interface{}{
			"errorMain":    "Sorry, the document cannot be opened",
			"errorSubtext": "Please try again",
			"reloadButton": "Reload",
		}

		var state request.DriveState
		if err := json.Unmarshal([]byte(qstate), &state); err != nil {
			c.logger.Debug("could not unmarshal state")
			http.Redirect(rw, r, "https://drive.google.com", http.StatusMovedPermanently)
			return
		}

		session, err := c.store.Get(r, "onlyoffice-auth")
		if err != nil {
			c.logger.Debugf("could not get auth session: %s", err.Error())
			http.Redirect(rw, r.WithContext(r.Context()), c.credentials.AuthCodeURL(
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
		errMsg = map[string]interface{}{
			"errorMain": loc.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "errorMain",
			}),
			"errorSubtext": loc.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "errorSubtext",
			}),
			"reloadButton": loc.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "reloadButton",
			}),
		}

		var ures response.UserResponse
		if err := c.client.Call(tctx, c.client.NewRequest(
			fmt.Sprintf("%s:auth", c.server.Namespace), "UserSelectHandler.GetUser",
			fmt.Sprint(state.UserID),
		), &ures); err != nil {
			c.logger.Debugf("could not get user %s access info: %s", state.UserID, err.Error())
			embeddable.ErrorPage.ExecuteTemplate(rw, "error", errMsg)
			return
		}

		srv, err := drive.NewService(tctx, option.WithHTTPClient(
			c.credentials.Client(tctx, &oauth2.Token{
				AccessToken:  ures.AccessToken,
				TokenType:    ures.TokenType,
				RefreshToken: ures.RefreshToken,
			})),
		)

		if err != nil {
			c.logger.Errorf("unable to retrieve drive service: %v", err)
			embeddable.ErrorPage.ExecuteTemplate(rw, "error", errMsg)
			return
		}

		file, err := srv.Files.Get(state.IDS[0]).Do()
		if err != nil {
			c.logger.Errorf("could not get file %s: %s", state.IDS[0], err.Error())
			embeddable.ErrorPage.ExecuteTemplate(rw, "error", errMsg)
			return
		}

		c.logger.Debugf("successfully found file with id %s", file.Id)
		_, gdriveFile := shared.GdriveMimeOnlyofficeExtension[file.MimeType]
		if c.fileUtil.IsExtensionEditable(file.FileExtension) || c.fileUtil.IsExtensionViewOnly(file.FileExtension) || gdriveFile {
			http.Redirect(rw, r, fmt.Sprintf("/editor?state=%s", qstate), http.StatusMovedPermanently)
			return
		}

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
					url.QueryEscape(string(state.ToJSON())),
				),
				http.StatusMovedPermanently,
			)
			return
		case "view":
			http.Redirect(
				rw, r,
				fmt.Sprintf(
					"/editor?state=%s",
					url.QueryEscape(string(state.ToJSON())),
				),
				http.StatusMovedPermanently,
			)
		case "create":
			nstate, err := c.convertFile(r.Context(), &state)
			if err != nil {
				rw.WriteHeader(http.StatusInternalServerError)
				return
			}
			http.Redirect(
				rw, r,
				fmt.Sprintf(
					"/editor?state=%s",
					url.QueryEscape(string(nstate.ToJSON())),
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

func (c ConvertController) convertFile(ctx context.Context, state *request.DriveState) (*request.DriveState, error) {
	uctx, cancel := context.WithTimeout(ctx, time.Duration(c.onlyoffice.Onlyoffice.Callback.UploadTimeout)*time.Second)
	defer cancel()

	var ures response.UserResponse
	if err := c.client.Call(uctx, c.client.NewRequest(
		fmt.Sprintf("%s:auth", c.server.Namespace), "UserSelectHandler.GetUser",
		fmt.Sprint(state.UserID),
	), &ures); err != nil {
		c.logger.Errorf("could not get user %s access info: %s", state.UserID, err.Error())
		return nil, err
	}

	srv, err := drive.NewService(uctx, option.WithHTTPClient(c.credentials.Client(ctx, &oauth2.Token{
		AccessToken:  ures.AccessToken,
		TokenType:    ures.TokenType,
		RefreshToken: ures.RefreshToken,
	})))

	if err != nil {
		c.logger.Errorf("unable to retrieve drive service: %v", err)
		return nil, err
	}

	file, err := srv.Files.Get(state.IDS[0]).Do()
	if err != nil {
		c.logger.Errorf("could not get file %s: %s", state.IDS[0], err.Error())
		return nil, err
	}

	downloadToken := &request.DriveDownloadToken{
		UserID: state.UserID,
		FileID: state.IDS[0],
	}
	downloadToken.IssuedAt = jwt.NewNumericDate(time.Now())
	downloadToken.ExpiresAt = jwt.NewNumericDate(time.Now().Add(4 * time.Minute))
	tkn, err := c.jwtManager.Sign(c.credentials.ClientSecret, downloadToken)

	var cresp response.ConvertResponse
	fType, err := c.fileUtil.GetFileType(file.FileExtension)
	if err != nil {
		c.logger.Debugf("could not get file type: %s", err.Error())
		return nil, err
	}

	creq := request.ConvertRequest{
		Async:      false,
		Filetype:   fType,
		Outputtype: "ooxml",
		URL: fmt.Sprintf(
			"%s/api/download?token=%s", c.onlyoffice.Onlyoffice.Builder.GatewayURL,
			tkn,
		),
	}
	creq.IssuedAt = jwt.NewNumericDate(time.Now())
	creq.ExpiresAt = jwt.NewNumericDate(time.Now().Add(2 * time.Minute))
	ctok, err := c.jwtManager.Sign(c.onlyoffice.Onlyoffice.Builder.DocumentServerSecret, creq)
	if err != nil {
		return nil, err
	}

	creq.Token = ctok
	req, err := http.NewRequestWithContext(
		uctx,
		"POST",
		fmt.Sprintf("%s/ConvertService.ashx", c.onlyoffice.Onlyoffice.Builder.DocumentServerURL),
		bytes.NewBuffer(creq.ToJSON()),
	)

	if err != nil {
		c.logger.Debugf("could not build a conversion api request: %s", err.Error())
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	resp, err := otelhttp.DefaultClient.Do(req)
	if err != nil {
		c.logger.Errorf("could not send a conversion api request: %s", err.Error())
		return nil, err
	}

	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(&cresp); err != nil {
		c.logger.Errorf("could not decode convert response body: %s", err.Error())
		return nil, err
	}

	cfile, err := otelhttp.Get(uctx, cresp.FileURL)
	if err != nil {
		c.logger.Errorf("could not retreive a converted file: %s", err.Error())
		return nil, err
	}

	defer cfile.Body.Close()
	now := time.Now().Format(time.RFC3339)
	filename := fmt.Sprintf("%s.%s", file.Title[:len(file.Title)-len(filepath.Ext(file.Title))], cresp.FileType)
	file, err = srv.Files.Insert(&drive.File{
		DriveId:                      file.DriveId,
		CreatedDate:                  now,
		ModifiedDate:                 now,
		ModifiedByMeDate:             now,
		Capabilities:                 file.Capabilities,
		ContentRestrictions:          file.ContentRestrictions,
		CopyRequiresWriterPermission: file.CopyRequiresWriterPermission,
		DefaultOpenWithLink:          file.DefaultOpenWithLink,
		Description:                  file.Description,
		FileExtension:                cresp.FileType,
		OriginalFilename:             filename,
		OwnedByMe:                    true,
		Title:                        filename,
		Parents:                      file.Parents,
		MimeType:                     mime.TypeByExtension(fmt.Sprintf(".%s", cresp.FileType)),
	}).Context(uctx).Media(cfile.Body).Do()

	if err != nil {
		c.logger.Errorf("could not modify file %s: %s", state.IDS[0], err.Error())
		return nil, err
	}

	return &request.DriveState{
		IDS:       []string{file.Id},
		Action:    state.Action,
		UserID:    state.UserID,
		UserAgent: state.UserAgent,
	}, nil
}
