// questo file per momento è useless
package main

import (
	"github.com/fxamacker/cbor/v2"
	"go.opentelemetry.io/otel"
	"log"
	"log/slog"
	"net/http"
	"sync"

)

// Global in-memory cache for metrics
var (
	globalMetricCache = make(map[string]Metrics)
	cacheMu           sync.RWMutex
)

// Convert temperature to a severity string
func tempToSeverityString(temp float64) string {
	switch {
	case temp < 75:
		return "INFO"
	case temp < 85:
		return "WARNING"
	case temp < 95:
		return "CRITICAL"
	case temp <= 100:
		return "EMERGENCY"
	default:
		return "INFO"
	}
}

// Convert temperature to a human-readable message
func tempToMessage(temp float64) string {
	switch {
	case temp < 75:
		return "Temperature is fine"
	case temp < 85:
		return "Temperature rising – monitor closely"
	case temp < 95:
		return "Critical temperature – action needed"
	case temp <= 100:
		return "Emergency – device may fail"
	default:
		return "Temperature is fine"
	}
}

// HTTP handler for receiving and logging device metrics
func handleMetrics(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	ctx, span := otel.Tracer("http-server").Start(r.Context(), "handleMetrics")
	defer span.End()

	var m Metrics

	// Decode the CBOR payload into the Metrics struct
	if err := cbor.NewDecoder(r.Body).Decode(&m); err != nil {
		log.Printf("CBOR decode error: %v", err)
		http.Error(w, "Invalid CBOR", http.StatusBadRequest)
		return
	}
	// Update the in-memory cache with the latest metrics
	updateMetricCache(m)

	// Determine severity and log the metric
	severityStr := tempToSeverityString(m.MCUTempC)
	level := mapSeverityToLevel(severityStr)

	slog.LogAttrs(ctx, level, tempToMessage(m.MCUTempC),
		slog.String("device_id", m.DeviceID),
		slog.Float64("value", m.MCUTempC),
		slog.String("type", "devicemetric"),
	)

	w.WriteHeader(http.StatusAccepted)
}

// Save or update the latest metric in the cache
func updateMetricCache(m Metrics) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	globalMetricCache[m.DeviceID] = m
}
