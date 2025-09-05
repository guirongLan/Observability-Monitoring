package main

import (
	"context"
	"log"
	"log/slog"
	"os"

	"github.com/plgd-dev/go-coap/v3/mux"
	coap "github.com/plgd-dev/go-coap/v3"
	//"go.opentelemetry.io/otel"
)

// startCoapServer starts the CoAP server with the given context.
// It reads the port from the environment variable "PORT", defaults to 5683 if not set.
// Then it creates a new CoAP router, registers routes, logs server start info, and listens.
func startCoapServer(ctx context.Context) {
	port := os.Getenv("PORT")
	if port == "" {
		port = "5683" // Default CoAP port
	}
	addr := ":" + port

	// Create a new CoAP router
	router := mux.NewRouter()
	registerCoapRoutes(router)

	slog.InfoContext(ctx, "Starting CoAP server", slog.String("addr", "0.0.0.0"+addr))

	// Start CoAP UDP server using coap.ListenAndServe
	// Use "udp" protocol since your client is using UDP
	log.Fatal(coap.ListenAndServe("udp", addr, router))
}

// registerCoapRoutes registers all CoAP routes to the provided router.
func registerCoapRoutes(router *mux.Router) {
	// Register handlers for batch log and metric endpoints
	router.Handle("/batchLog", mux.HandlerFunc(handleCoapBatchLog))
	router.Handle("/batchMetric", mux.HandlerFunc(handleCoapMetrics))
	
	slog.Info("Registered CoAP routes: /batchLog, /batchMetric")
}