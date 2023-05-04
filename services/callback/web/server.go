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

	chttp "github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/service/http"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/worker"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/callback/web/controller"
	workerh "github.com/ONLYOFFICE/onlyoffice-gdrive/services/callback/web/worker"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

type CallbackService struct {
	mux               *chi.Mux
	worker            worker.BackgroundWorker
	cbworker          workerh.CallbackWorker
	enqueuer          worker.BackgroundEnqueuer
	callbackConroller controller.CallbackController
}

// ApplyMiddleware useed to apply http server middlewares.
func (s CallbackService) ApplyMiddleware(middlewares ...func(http.Handler) http.Handler) {
	s.mux.Use(middlewares...)
}

// NewService initializes http server with options.
func NewServer(
	wrkr worker.BackgroundWorker,
	cbworker workerh.CallbackWorker,
	enqueuer worker.BackgroundEnqueuer,
	callbackController controller.CallbackController,
) chttp.ServerEngine {
	service := CallbackService{
		mux:               chi.NewRouter(),
		worker:            wrkr,
		cbworker:          cbworker,
		enqueuer:          enqueuer,
		callbackConroller: callbackController,
	}

	return service
}

// NewHandler returns http server engine.
func (s CallbackService) NewHandler() interface {
	ServeHTTP(w http.ResponseWriter, r *http.Request)
} {
	return s.InitializeServer()
}

// InitializeServer sets all injected dependencies.
func (s *CallbackService) InitializeServer() *chi.Mux {
	s.worker.Register("gdrive-callback-upload", s.cbworker.UploadFile)
	s.InitializeRoutes()
	s.worker.Run()
	return s.mux
}

// InitializeRoutes builds all http routes.
func (s *CallbackService) InitializeRoutes() {
	s.mux.Group(func(r chi.Router) {
		r.Use(chimiddleware.Recoverer)
		r.NotFound(func(rw http.ResponseWriter, r *http.Request) {
			http.Redirect(rw, r.WithContext(r.Context()), "https://onlyoffice.com", http.StatusMovedPermanently)
		})
		r.Post("/callback", s.callbackConroller.BuildPostHandleCallback(s.enqueuer))
	})
}
