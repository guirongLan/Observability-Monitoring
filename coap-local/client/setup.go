package main

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/propagation"
)

// setupTracer initializes OpenTelemetry tracing system and sets up a tracer provider.
func setupTracer() (shutdown func(context.Context) error, err error) {
	// Create a new TracerProvider.
	tp := trace.NewTracerProvider()
	// Set the created TracerProvider as the global provider
	otel.SetTracerProvider(tp)
	// Use W3C Trace Context propagation
	otel.SetTextMapPropagator(propagation.TraceContext{})
	return tp.Shutdown, nil
}
