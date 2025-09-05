package main

import (
	"github.com/fxamacker/cbor/v2"
	"github.com/plgd-dev/go-coap/v3/message"
	"github.com/plgd-dev/go-coap/v3/message/codes"
	"github.com/plgd-dev/go-coap/v3/mux"
	"go.opentelemetry.io/otel"
	"log"
	"log/slog"
	"sync"
	"time"
)

// Global in-memory cache for metrics
var (
	globalMetricCache = make(map[string]Metrics)
	cacheMu           sync.RWMutex
)

// Metrics defines the structure for device metrics
type Metrics struct {
	DeviceID         string    `cbor:"device_id"`
	Timestamp        time.Time `cbor:"timestamp"`
	CPUPercent       float64   `cbor:"cpu_percent"`
	MemUsedMB        float64   `cbor:"mem_used_mb"`
	TempC            float64   `cbor:"temp_c"`
	DiskUsagePercent float64   `cbor:"disk_usage_percent"`
	DiskReadMBps     float64   `cbor:"disk_read_mbps"`
	DiskWriteMBps    float64   `cbor:"disk_write_mbps"`
}

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

// CoAP handler for receiving and logging device metrics
func handleCoapMetrics(w mux.ResponseWriter, r *mux.Message) {
	ctx, span := otel.Tracer("coap-server").Start(r.Context(), "handleCoapMetrics")
	defer span.End()

	var m Metrics

	// Get the message body
	body, err := r.ReadBody()
	if err != nil {
		log.Printf("Error reading CoAP message body: %v", err)
		w.SetResponse(codes.BadRequest, message.TextPlain, nil)
		return
	}

	// Decode the CBOR payload into the Metrics struct
	if err := cbor.Unmarshal(body, &m); err != nil {
		log.Printf("CBOR decode error: %v", err)
		w.SetResponse(codes.BadRequest, message.TextPlain, nil)
		return
	}

	// Update the in-memory cache with the latest metrics
	updateMetricCache(m)

	// Determine severity and log the metric
	severityStr := tempToSeverityString(m.TempC)
	level := mapSeverityToLevel(severityStr)

	slog.LogAttrs(ctx, level, tempToMessage(m.TempC),
		slog.String("device_id", m.DeviceID),
		slog.Float64("value", m.TempC),
		slog.String("type", "devicemetric"),
	)

	// Send CoAP 2.04 Changed response to confirm successful processing
	w.SetResponse(codes.Changed, message.TextPlain, nil)
}

// Save or update the latest metric in the cache
func updateMetricCache(m Metrics) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	globalMetricCache[m.DeviceID] = m
}