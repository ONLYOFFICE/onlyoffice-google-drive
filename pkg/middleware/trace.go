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

package middleware

import (
	"net/http"

	"go-micro.dev/v4/metadata"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

const (
	InstrumentationName = "github.com/go-micro/plugins/v4/wrapper/trace/opentelemetry"
)

// TracePropagationMiddleware inject previous context into a new request
func TracePropagationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		carrier := propagation.MapCarrier{}
		propagator := otel.GetTextMapPropagator()
		ctx := propagator.Extract(r.Context(), carrier)

		propagator.Inject(ctx, carrier)
		next.ServeHTTP(w, r.WithContext(metadata.NewContext(ctx, metadata.Metadata(carrier))))
	})
}

// LogTraceMiddleware starts tracing with spans
func LogTraceMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		carrier := propagation.MapCarrier{}

		ctx, span := otel.GetTracerProvider().Tracer(InstrumentationName).Start(r.Context(), r.URL.Path)
		defer span.End()

		otel.GetTextMapPropagator().Inject(ctx, carrier)
		next.ServeHTTP(w, r.WithContext(metadata.NewContext(ctx, metadata.Metadata(carrier))))
	})
}
