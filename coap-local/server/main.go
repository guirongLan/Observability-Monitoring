package main

import (
	"context"
	"go.opentelemetry.io/otel"
	"log"
	"log/slog"
	"os"
)

func main() {
	// Create a root context for the application lifecycle
	ctx := context.Background()
	// Initialize logging system (custom setup function)
	setupLogging()

	// Initialize OpenTelemetry tracing and metrics
	shutdown, err := setupOpentelemetry(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "error setting up OpenTelemetry", slog.Any("error", err))
		os.Exit(1)
	}
	// Ensure OpenTelemetry resources are properly cleaned up on exit
	defer shutdown(ctx)

	// Retrieve a Meter instance named "http-server" from the global OpenTelemetry MeterProvider
	// Meter is used to create and manage metrics instruments
	// Naming the Meter "http-server" helps identify the source of metrics in visualization tools like Grafana
	meter = otel.GetMeterProvider().Meter("http-server")

	// Initialize metrics instruments (e.g., counters, gauges) with the Meter
	initMetrics(meter)

	// Register all gauge observers that read data from the globalMetricCache
	// Observers periodically collect metric values for reporting
	if err := registerObservers(meter); err != nil {
		log.Fatalf("failed to register observers: %v", err)
	}
	// Start the HTTP server which will handle incoming requests
	startCoapServer(ctx)
}
