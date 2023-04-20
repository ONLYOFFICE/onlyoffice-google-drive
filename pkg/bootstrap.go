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

package pkg

import (
	"context"
	"net/http"
	"os"

	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/cache"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/client"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/config"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/crypto"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/log"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/messaging"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/onlyoffice"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/registry"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/service/repl"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/trace"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/worker"
	"go-micro.dev/v4"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"golang.org/x/sync/errgroup"
)

func Bootstrap(path string, extras ...interface{}) *fx.App {
	builder := config.BuildNewServerConfig(path)
	sconf, err := builder()
	if err != nil {
		log := log.NewDefaultLogger(&config.LoggerConfig{})
		log.Fatal(err.Error())
		return nil
	}

	var logger fx.Option = fx.NopLogger
	if sconf.Debug {
		logger = fx.WithLogger(func() fxevent.Logger {
			return &fxevent.ConsoleLogger{W: os.Stdout}
		})
	}

	return fx.New(
		fx.Provide(config.BuildNewCacheConfig(path)),
		fx.Provide(config.BuildNewCorsConfig(path)),
		fx.Provide(config.BuildNewLoggerConfig(path)),
		fx.Provide(config.BuildNewMessagingConfig(path)),
		fx.Provide(config.BuildNewPersistenceConfig(path)),
		fx.Provide(config.BuildNewRegistryConfig(path)),
		fx.Provide(config.BuildNewResilienceConfig(path)),
		fx.Provide(builder),
		fx.Provide(config.BuildNewTracerConfig(path)),
		fx.Provide(config.BuildNewWorkerConfig(path)),
		fx.Provide(config.BuildNewCryptoConfig(path)),
		fx.Provide(cache.NewCache),
		fx.Provide(log.NewLogrusLogger),
		fx.Provide(registry.NewRegistry),
		fx.Provide(messaging.NewBroker),
		fx.Provide(client.NewClient),
		fx.Provide(trace.NewTracer),
		fx.Provide(worker.NewBackgroundWorker),
		fx.Provide(worker.NewBackgroundEnqueuer),
		fx.Provide(repl.NewService),
		fx.Provide(crypto.NewEncryptor),
		fx.Provide(crypto.NewJwtManager),
		fx.Provide(crypto.NewHasher),
		fx.Provide(onlyoffice.NewOnlyofficeFileUtility),
		fx.Provide(extras...),
		fx.Invoke(func(lifecycle fx.Lifecycle, service micro.Service, repl *http.Server, logger log.Logger) {
			lifecycle.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					go repl.ListenAndServe()
					go service.Run()
					return nil
				},
				OnStop: func(ctx context.Context) error {
					g, gCtx := errgroup.WithContext(ctx)
					g.Go(func() error {
						return repl.Shutdown(gCtx)
					})
					return g.Wait()
				},
			})
		}),
		logger,
	)
}
