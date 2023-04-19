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

package trace

import (
	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

type TracerType int

var (
	Default TracerType = 0
	Zipkin  TracerType = 1
)

// NewTracer initializes a new tracer.
func NewTracer(config *config.TracerConfig) (*trace.TracerProvider, error) {
	var exporter trace.SpanExporter

	if config.Tracer.Name == "" {
		config.Tracer.Name = "default-tracer"
	}

	switch config.Tracer.TracerType {
	case 1:
		if config.Tracer.Address == "" {
			return nil, ErrTracerInvalidAddressInitialization
		}
		exporter = NewZipkinExporter(config.Tracer.Address)
	default:
		exporter, _ = stdouttrace.New()
	}

	provider := trace.NewTracerProvider(
		trace.WithSampler(trace.ParentBased(trace.TraceIDRatioBased(config.Tracer.FractionRatio))),
		trace.WithBatcher(exporter),
		trace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(config.Tracer.Name),
		)),
	)

	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	return provider, nil
}
