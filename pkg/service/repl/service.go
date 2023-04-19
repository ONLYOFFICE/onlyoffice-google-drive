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

package repl

import (
	"fmt"
	"net/http"
	"net/http/pprof"
	"strconv"
	"time"

	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/config"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/middleware"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/hellofresh/health-go/v5"
	"github.com/justinas/alice"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// NewService Initializes repl service with options.
func NewService(
	replConfig *config.ServerConfig,
	corsConfig *config.CORSConfig,
) *http.Server {
	mux := http.NewServeMux()
	h, _ := health.New(health.WithComponent(health.Component{
		Name:    fmt.Sprintf("%s:%s", replConfig.Namespace, replConfig.Name),
		Version: fmt.Sprintf("v%d", replConfig.Version),
	}))

	mux.Handle("/metrics", promhttp.Handler())
	mux.Handle("/health", h.Handler())

	if replConfig.Debug {
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	}

	return &http.Server{
		Addr: replConfig.ReplAddress,
		Handler: alice.New(
			chimiddleware.RealIP,
			middleware.NewRateLimiter(1000, 1*time.Second, middleware.WithKeyFuncAll),
			chimiddleware.RequestID,
			middleware.Cors(corsConfig.CORS.AllowedOrigins, corsConfig.CORS.AllowedMethods, corsConfig.CORS.AllowedHeaders, corsConfig.CORS.AllowCredentials),
			middleware.Secure,
			middleware.NoCache,
			middleware.Version(strconv.Itoa(replConfig.Version)),
		).Then(mux),
	}
}
