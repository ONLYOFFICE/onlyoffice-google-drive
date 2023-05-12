package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/config"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/crypto"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/log"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/gateway/web/embeddable"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared/request"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared/response"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/sessions"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"go-micro.dev/v4/client"
	"golang.org/x/oauth2"
)

type EditorController struct {
	client     client.Client
	jwtManager crypto.JwtManager
	store      *sessions.CookieStore
	server     *config.ServerConfig
	oauth      *oauth2.Config
	logger     log.Logger
}

func NewEditorController(
	client client.Client,
	jwtManager crypto.JwtManager,
	server *config.ServerConfig,
	oauth *oauth2.Config,
	logger log.Logger,
) EditorController {
	return EditorController{
		client:     client,
		jwtManager: jwtManager,
		store:      sessions.NewCookieStore([]byte(oauth.ClientSecret)),
		server:     server,
		oauth:      oauth,
		logger:     logger,
	}
}

func (c EditorController) BuildGetEditor() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "text/html")
		qstate := r.URL.Query().Get("state")
		state := request.DriveState{
			UserAgent: r.UserAgent(),
		}

		if err := json.Unmarshal([]byte(qstate), &state); err != nil {
			http.Redirect(rw, r, "https://drive.google.com/", http.StatusMovedPermanently)
			return
		}

		session, _ := c.store.Get(r, state.UserID)
		val, ok := session.Values["token"].(string)
		if !ok {
			c.logger.Debugf("could not cast a session jwt")
			session.Options.MaxAge = -1
			session.Save(r, rw)
			http.Redirect(rw, r.WithContext(r.Context()), c.oauth.AuthCodeURL(
				"state-token", oauth2.AccessTypeOffline, oauth2.ApprovalForce,
			), http.StatusSeeOther)
			return
		}

		var token jwt.RegisteredClaims
		if err := c.jwtManager.Verify(c.oauth.ClientSecret, val, &token); err != nil {
			c.logger.Warnf("could not verify a jwt: %s", err.Error())
			http.Redirect(rw, r.WithContext(r.Context()), c.oauth.AuthCodeURL(
				"state-token", oauth2.AccessTypeOffline, oauth2.ApprovalForce,
			), http.StatusSeeOther)
			return
		}

		val, _ = session.Values["locale"].(string)
		loc := i18n.NewLocalizer(embeddable.Bundle, val)
		var resp response.BuildConfigResponse
		if err := c.client.Call(r.Context(),
			c.client.NewRequest(fmt.Sprintf("%s:builder", c.server.Namespace), "ConfigHandler.BuildConfig", state),
			&resp,
		); err != nil {
			c.logger.Errorf("could not build onlyoffice config: %s", err.Error())
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				rw.WriteHeader(http.StatusRequestTimeout)
				return
			}

			microErr := response.MicroError{}
			if err := json.Unmarshal([]byte(err.Error()), &microErr); err != nil {
				rw.WriteHeader(http.StatusInternalServerError)
				return
			}

			c.logger.Errorf("build config micro error: %s", microErr.Detail)
			embeddable.ErrorPage.Execute(rw, map[string]interface{}{
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
			"apijs":   fmt.Sprintf("%s/web-apps/apps/api/documents/api.js", resp.ServerURL),
			"config":  string(resp.ToJSON()),
			"docType": resp.DocumentType,
			"cancelButton": loc.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "cancelButton",
			}),
		})
	}
}
