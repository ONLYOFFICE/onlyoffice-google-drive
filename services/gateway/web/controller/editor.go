package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path"
	"path/filepath"
	"runtime"
	"text/template"
	"time"

	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/config"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/crypto"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/log"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared/request"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared/response"
	"github.com/golang-jwt/jwt"
	"github.com/gorilla/sessions"
	"go-micro.dev/v4/client"
	"golang.org/x/oauth2"
)

var (
	_, b, _, _ = runtime.Caller(0)
	basepath   = filepath.Dir(b)
	editorPage = template.Must(template.ParseFiles(path.Join(basepath, "../", "templates", "editor.html")))
	errorPage  = template.Must(template.ParseFiles(path.Join(basepath, "../", "templates", "error.html")))
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
			errorPage.Execute(rw, nil)
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

		var token jwt.StandardClaims
		if err := c.jwtManager.Verify(c.oauth.ClientSecret, val, &token); err != nil {
			c.logger.Warnf("could not verify a jwt: %s", err.Error())
			http.Redirect(rw, r.WithContext(r.Context()), c.oauth.AuthCodeURL(
				"state-token", oauth2.AccessTypeOffline, oauth2.ApprovalForce,
			), http.StatusSeeOther)
			return
		}

		c.logger.Debugf("jwt %s is valid", val)

		signature, err := c.jwtManager.Sign(c.oauth.ClientSecret, jwt.StandardClaims{
			Id:        state.UserID,
			ExpiresAt: time.Now().Add(24 * 7 * time.Hour).UnixMilli(),
		})

		if err != nil {
			errorPage.Execute(rw, nil)
			return
		}

		c.logger.Debugf("successfully issued a new jwt: %s", signature)

		req := c.client.NewRequest(
			fmt.Sprintf("%s:auth", c.server.Namespace), "UserSelectHandler.GetUser",
			fmt.Sprint(state.UserID),
		)

		var ures response.UserResponse
		if err := c.client.Call(r.Context(), req, &ures); err != nil {
			c.logger.Debugf("could not get user %s access info: %s", state.UserID, err.Error())
			errorPage.Execute(rw, nil)
			return
		}

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
			errorPage.Execute(rw, nil)
			return
		}

		session.Values["token"] = signature
		session.Options.MaxAge = 60 * 60 * 23 * 7
		if err := session.Save(r, rw); err != nil {
			c.logger.Errorf("could not save a new session cookie: %s", err.Error())
			errorPage.Execute(rw, nil)
			return
		}

		c.logger.Debug("successfully saved a new session cookie")

		editorPage.Execute(rw, map[string]interface{}{
			"apijs":   fmt.Sprintf("%s/web-apps/apps/api/documents/api.js", resp.ServerURL),
			"config":  string(resp.ToJSON()),
			"docType": resp.DocumentType,
		})
	}
}
