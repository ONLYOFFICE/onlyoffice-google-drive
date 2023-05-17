package middleware

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/crypto"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/log"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared/request"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/sessions"
	"go-micro.dev/v4/logger"
	"golang.org/x/oauth2"
)

type SessionMiddleware struct {
	jwtManager  crypto.JwtManager
	store       *sessions.CookieStore
	credentials *oauth2.Config
	logger      log.Logger
}

func NewSessionMiddleware(
	jwtManager crypto.JwtManager,
	credentials *oauth2.Config,
	logger log.Logger,
) SessionMiddleware {
	return SessionMiddleware{
		jwtManager:  jwtManager,
		store:       sessions.NewCookieStore([]byte(credentials.ClientSecret)),
		credentials: credentials,
		logger:      logger,
	}
}

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
			session.Save(r, rw)
			http.Redirect(rw, r.WithContext(r.Context()), m.credentials.AuthCodeURL(
				"state-token", oauth2.AccessTypeOffline, oauth2.ApprovalForce,
			), http.StatusSeeOther)
			return
		}

		var token jwt.MapClaims
		if err := m.jwtManager.Verify(m.credentials.ClientSecret, val, &token); err != nil {
			m.logger.Debugf("could not verify session token: %s", err.Error())
			session.Options.MaxAge = -1
			session.Save(r, rw)
			http.Redirect(rw, r.WithContext(r.Context()), m.credentials.AuthCodeURL(
				"state-token", oauth2.AccessTypeOffline, oauth2.ApprovalForce,
			), http.StatusSeeOther)
			return
		}

		if token["jti"] != state.UserID {
			m.logger.Debugf("user % doesn't match state user %s", token["jti"], state.UserID)
			session.Options.MaxAge = -1
			session.Save(r, rw)
			http.Redirect(rw, r.WithContext(r.Context()), m.credentials.AuthCodeURL(
				"state-token", oauth2.AccessTypeOffline, oauth2.ApprovalForce,
			), http.StatusSeeOther)
			return
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

		next.ServeHTTP(rw, r)
	}

	return http.HandlerFunc(fn)
}
