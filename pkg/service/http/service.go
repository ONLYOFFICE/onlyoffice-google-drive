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

package http

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/config"
	plog "github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/log"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/messaging"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/middleware"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/resilience"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	mserver "github.com/go-micro/plugins/v4/server/http"
	"github.com/go-micro/plugins/v4/wrapper/breaker/hystrix"
	"github.com/go-micro/plugins/v4/wrapper/select/roundrobin"
	"github.com/go-micro/plugins/v4/wrapper/trace/opentelemetry"
	"go-micro.dev/v4"
	"go-micro.dev/v4/cache"
	"go-micro.dev/v4/client"
	"go-micro.dev/v4/registry"
	"go-micro.dev/v4/server"
	"go.opentelemetry.io/otel"
	oteltrace "go.opentelemetry.io/otel/sdk/trace"
)

type ServerEngine interface {
	ApplyMiddleware(middlewares ...func(http.Handler) http.Handler)
	NewHandler() interface {
		ServeHTTP(w http.ResponseWriter, req *http.Request)
	}
}

// NewService Initializes an http service.
func NewService(
	engine ServerEngine,
	client client.Client,
	registry registry.Registry,
	broker messaging.BrokerWithOptions,
	cache cache.Cache,
	tracer *oteltrace.TracerProvider,
	logger plog.Logger,
	serverConfig *config.ServerConfig,
	resilienceConfig *config.ResilienceConfig,
	corsConfig *config.CORSConfig,
	tracerConfig *config.TracerConfig,
) micro.Service {
	if err := broker.Broker.Init(); err != nil {
		log.Fatalf("could not initialize a new broker instance: %s", err.Error())
	}

	if err := broker.Broker.Connect(); err != nil {
		log.Fatalf("broker connection error: %s", err.Error())
	}

	hystrix.ConfigureDefault(resilience.BuildHystrixCommandConfig(resilienceConfig))

	service := micro.NewService(
		micro.Name(strings.Join([]string{serverConfig.Namespace, serverConfig.Name}, ":")),
		micro.Version(strconv.Itoa(serverConfig.Version)),
		micro.Context(context.Background()),
		micro.Server(mserver.NewServer(
			server.Name(strings.Join([]string{serverConfig.Namespace, serverConfig.Name}, ":")),
			server.Address(serverConfig.Address),
		)),
		micro.Cache(cache),
		micro.Registry(registry),
		micro.Broker(broker.Broker),
		micro.Client(client),
		micro.WrapClient(
			roundrobin.NewClientWrapper(),
			hystrix.NewClientWrapper(),
		),
		micro.WrapCall(opentelemetry.NewCallWrapper(opentelemetry.WithTraceProvider(otel.GetTracerProvider()))),
		micro.RegisterTTL(30*time.Second),
		micro.RegisterInterval(10*time.Second),
		micro.AfterStop(func() error {
			if tracer != nil {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()

				if err := tracer.Shutdown(ctx); err != nil {
					return err
				}

				return nil
			}

			return nil
		}),
	)

	if resilienceConfig.Resilience.RateLimiter.IPLimit > 0 {
		engine.ApplyMiddleware(middleware.NewRateLimiter(resilienceConfig.Resilience.RateLimiter.IPLimit, 1*time.Second, middleware.WithKeyFuncIP))
	}

	if resilienceConfig.Resilience.RateLimiter.Limit > 0 {
		engine.ApplyMiddleware(middleware.NewRateLimiter(resilienceConfig.Resilience.RateLimiter.Limit, 1*time.Second, middleware.WithKeyFuncAll))
	}

	engine.ApplyMiddleware(
		middleware.Log(logger),
		chimiddleware.RealIP,
		chimiddleware.RequestID,
		chimiddleware.StripSlashes,
		middleware.Secure,
		middleware.Version(strconv.Itoa(serverConfig.Version)),
		middleware.Cors(corsConfig.CORS.AllowedOrigins, corsConfig.CORS.AllowedMethods, corsConfig.CORS.AllowedHeaders, corsConfig.CORS.AllowCredentials),
	)

	if tracerConfig.Tracer.Enable {
		engine.ApplyMiddleware(
			middleware.TracePropagationMiddleware,
			middleware.LogTraceMiddleware,
		)
	}

	if err := micro.RegisterHandler(
		service.Server(),
		engine.NewHandler(),
	); err != nil {
		log.Fatal("could not register http handler: ", err)
	}

	return service
}
