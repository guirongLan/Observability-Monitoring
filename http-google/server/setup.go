package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/trace"
	//"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	//"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
)

// setupOpentelemetry configures OpenTelemetry tracing and metrics exporters to send data
// to a remote OpenTelemetry Collector. It returns a shutdown function to clean up resources.
func setupOpentelemetry(ctx context.Context) (shutdown func(context.Context) error, err error) {
	var shutdownFuncs []func(context.Context) error

	// shutdown function calls all registered shutdown functions in sequence and joins errors
	shutdown = func(ctx context.Context) error {
		var err error
		for _, fn := range shutdownFuncs {
			err = errors.Join(err, fn(ctx))
		}
		shutdownFuncs = nil
		return err
	}

	// Set the global propagator to TraceContext for trace context propagation over HTTP
	otel.SetTextMapPropagator(propagation.TraceContext{})

	// Create a new OTLP trace exporter sending to a specific endpoint and URL path of the collector
	tExporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint("otel-collector-1094805005874.europe-west1.run.app"),
		otlptracehttp.WithURLPath("/v1/traces"),
	)
	if err != nil {
		err = errors.Join(err, shutdown(ctx))
		return
	}

	// Create a tracer provider using the trace exporter and batch processing
	tp := trace.NewTracerProvider(trace.WithBatcher(tExporter))
	shutdownFuncs = append(shutdownFuncs, tp.Shutdown)
	// Set the global tracer provider for the application
	otel.SetTracerProvider(tp)

	// Create a new OTLP metric exporter to the same collector endpoint for metrics
	mExporter, err := otlpmetrichttp.New(ctx,
		otlpmetrichttp.WithEndpoint("otel-collector-1094805005874.europe-west1.run.app"),
		otlpmetrichttp.WithURLPath("/v1/metrics"),
	)
	if err != nil {
		err = errors.Join(err, shutdown(ctx))
		return
	}

	// Create a metric provider with a periodic reader that exports metrics every 1 minute
	mp := metric.NewMeterProvider(
		metric.WithReader(
			metric.NewPeriodicReader(mExporter,
				metric.WithInterval(1*time.Minute), // Export metrics every 1 minute
			),
		),
	)
	shutdownFuncs = append(shutdownFuncs, mp.Shutdown)

	// Set the global meter provider for metrics
	otel.SetMeterProvider(mp)

	return shutdown, nil
}

// setupLogging configures structured JSON logging to stdout using slog,
// with log levels, attribute replacements for compatibility, and
// OpenTelemetry span context injected into logs.
func setupLogging() {
	// Create a JSON handler for slog that outputs to stdout and replaces attributes using replacer function
	jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:       slog.LevelDebug, // Log all levels >= Debug
		ReplaceAttr: replacer})	// Customize attribute keys and values
	
	// Wrap the handler so it automatically adds OpenTelemetry span context to each log record
	instrumentedHandler := handlerWithSpanContext(jsonHandler)
	
	// Set the default global logger to use this instrumented handler
	slog.SetDefault(slog.New(instrumentedHandler))
}
/*
// Alternative setup for pure local testing without any remote dependencies
func setupLocalOnlyTelemetry(ctx context.Context) (shutdown func(context.Context) error, err error) {
	var shutdownFuncs []func(context.Context) error

	// shutdown function calls all registered shutdown functions in sequence and joins errors
	shutdown = func(ctx context.Context) error {
		var err error
		for _, fn := range shutdownFuncs {
			err = errors.Join(err, fn(ctx))
		}
		shutdownFuncs = nil
		return err
	}

	// Set the global propagator to TraceContext for trace context propagation over HTTP
	otel.SetTextMapPropagator(propagation.TraceContext{})

	// Create stdout trace exporter for local development/testing
	tExporter, err := stdouttrace.New(
		stdouttrace.WithPrettyPrint(),
	)
	if err != nil {
		err = errors.Join(err, shutdown(ctx))
		return
	}

	// Create tracer provider with stdout exporter
	tp := trace.NewTracerProvider(trace.WithBatcher(tExporter))
	shutdownFuncs = append(shutdownFuncs, tp.Shutdown)

	// Set the global tracer provider for the application
	otel.SetTracerProvider(tp)

	// Create stdout metric exporter for local development/testing
	mExporter, err := stdoutmetric.New(
		stdoutmetric.WithPrettyPrint(),
	)
	if err != nil {
		err = errors.Join(err, shutdown(ctx))
		return
	}

	// Create metric provider with stdout exporter and shorter interval for testing
	mp := metric.NewMeterProvider(
		metric.WithReader(
			metric.NewPeriodicReader(mExporter,
				metric.WithInterval(15*time.Second), // Export metrics every 15 seconds for local testing
			),
		),
	)
	shutdownFuncs = append(shutdownFuncs, mp.Shutdown)

	// Set the global meter provider for metrics
	otel.SetMeterProvider(mp)

	return shutdown, nil
}*/