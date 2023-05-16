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

package web

import (
	"net/http"

	shttp "github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/service/http"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/gateway/web/controller"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/csrf"
	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"
)

type GdriveHTTPService struct {
	mux               *chi.Mux
	store             sessions.Store
	authController    controller.AuthController
	editorController  controller.EditorController
	fileController    controller.FileController
	convertController controller.ConvertController
	credentials       *oauth2.Config
}

// NewService initializes http server with options.
func NewServer(
	authController controller.AuthController,
	editorController controller.EditorController,
	fileController controller.FileController,
	convertController controller.ConvertController,
	credentialsConfig *shared.OAuthCredentialsConfig,
	credentials *oauth2.Config,
) shttp.ServerEngine {
	service := GdriveHTTPService{
		mux:               chi.NewRouter(),
		store:             sessions.NewCookieStore([]byte(credentialsConfig.Credentials.ClientSecret)),
		authController:    authController,
		editorController:  editorController,
		fileController:    fileController,
		convertController: convertController,
		credentials:       credentials,
	}

	return service
}

// ApplyMiddleware useed to apply http server middlewares.
func (s GdriveHTTPService) ApplyMiddleware(middlewares ...func(http.Handler) http.Handler) {
	s.mux.Use(middlewares...)
}

// NewHandler returns http server engine.
func (s GdriveHTTPService) NewHandler() interface {
	ServeHTTP(w http.ResponseWriter, r *http.Request)
} {
	return s.InitializeServer()
}

// InitializeServer sets all injected dependencies.
func (s *GdriveHTTPService) InitializeServer() *chi.Mux {
	s.InitializeRoutes()
	return s.mux
}

// InitializeRoutes builds all http routes.
func (s *GdriveHTTPService) InitializeRoutes() {
	fs := http.FileServer(http.Dir("services/gateway/static"))
	csrfMiddleware := csrf.Protect([]byte(s.credentials.ClientSecret))
	s.mux.Group(func(r chi.Router) {
		r.Use(chimiddleware.Recoverer, chimiddleware.NoCache, csrfMiddleware)

		r.Handle("/static/*", http.StripPrefix("/static/", fs))

		r.Route("/oauth", func(cr chi.Router) {
			cr.Get("/auth", s.authController.BuildGetAuth())
		})

		r.Route("/api", func(cr chi.Router) {
			cr.Get("/download", s.fileController.BuildDownloadFile())
			cr.Get("/editor", s.editorController.BuildGetEditor())
			cr.Get("/create", s.fileController.BuildCreateFilePage())
			cr.Post("/create", s.fileController.BuildCreateFile())
			cr.Get("/convert", s.convertController.BuildConvertPage())
			cr.Post("/convert", s.convertController.BuildConvertFile())
		})

		r.NotFound(func(rw http.ResponseWriter, cr *http.Request) {
			http.Redirect(rw, cr.WithContext(cr.Context()), s.credentials.AuthCodeURL(
				"state-token", oauth2.AccessTypeOffline, oauth2.ApprovalForce,
			), http.StatusMovedPermanently)
		})
	})
}
