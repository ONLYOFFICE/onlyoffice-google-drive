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

package wrapper

import (
	"context"
	"strings"

	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/middleware"
	"go-micro.dev/v4/metadata"
	"go-micro.dev/v4/server"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// TracePropagationHandlerWrapper wraps RPC handlers to trace rpc calls
func TracePropagationHandlerWrapper(hf server.HandlerFunc) server.HandlerFunc {
	return func(ctx context.Context, req server.Request, rsp interface{}) error {
		meta, _ := metadata.FromContext(ctx)
		converted := make(map[string]string)

		for k, v := range meta {
			converted[strings.ToLower(k)] = v
		}

		ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(converted))

		ctx, span := otel.GetTracerProvider().Tracer(middleware.InstrumentationName).Start(ctx, req.Endpoint())
		defer span.End()

		if err := hf(ctx, req, rsp); err != nil {
			return err
		}

		return nil
	}
}
