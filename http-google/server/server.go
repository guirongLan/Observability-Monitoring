package main

import (
	"context"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"log"
	"log/slog"
	"net/http"
	"os"
)

// registerRoutes registers all HTTP routes to the provided ServeMux (router).
// *http.ServeMux is Go's HTTP request multiplexer that matches URL paths to handlers.
// This function also wraps handlers with OpenTelemetry instrumentation for tracing.
func registerRoutes(mux *http.ServeMux) {
	registerInstrumentedRoute(mux, "/batchLog", handleBatchLog)
	registerInstrumentedRoute(mux, "/batchMetric", handleMetrics)
}

// startHTTPServer starts the HTTP server with the given context.
// It reads the port from the environment variable "PORT", defaults to 8080 if not set.
// Then it creates a new ServeMux, registers routes, logs server start info, and listens.
func startHTTPServer(ctx context.Context) {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port

	mux := http.NewServeMux()
	registerRoutes(mux)

	slog.InfoContext(ctx, "Starting HTTP server", slog.String("addr", "0.0.0.0"+addr))

	// Start HTTP server and log fatal error if it fails
	log.Fatal(http.ListenAndServe(addr, mux))
}

// registerInstrumentedRoute wraps the given HTTP handler with OpenTelemetry instrumentation
// so that each request is automatically traced and metrics are collected.
// It then registers the instrumented handler with the given route path on the mux.
func registerInstrumentedRoute(mux *http.ServeMux, route string, handler http.HandlerFunc) {
	// Wrap the handler with OpenTelemetry HTTP instrumentation, adding the route as a tag
	instrumentedHandler := otelhttp.NewHandler(otelhttp.WithRouteTag(route, handler), route)
	mux.Handle(route, instrumentedHandler)
}
