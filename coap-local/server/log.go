package main

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

// Define custom log severity levels compatible with GCP (Google Cloud Platform)
// These are in addition to the default slog levels.
const (
	LevelDebug     = slog.LevelDebug // -4
	LevelInfo      = slog.LevelInfo  // 0
	LevelNotice    = slog.Level(1)
	LevelWarning   = slog.LevelWarn  // 4
	LevelError     = slog.LevelError // 8
	LevelCritical  = slog.Level(10)
	LevelAlert     = slog.Level(12)
	LevelEmergency = slog.Level(14)
)

// Custom log handler that embeds span context (trace ID, span ID, sampling flag) into the log record
type spanContextLogHandler struct {
	slog.Handler
}

// Helper function that wraps an existing slog.Handler with span context support
func handlerWithSpanContext(handler slog.Handler) *spanContextLogHandler {
	return &spanContextLogHandler{Handler: handler}
}

// Override Handle method: enrich logs with OpenTelemetry trace info if available
func (t *spanContextLogHandler) Handle(ctx context.Context, record slog.Record) error {
	// Extract the current trace context (if present) from the context
	if s := trace.SpanContextFromContext(ctx); s.IsValid() {
		// Add trace ID to the log
		record.AddAttrs(
			slog.Any("logging.googleapis.com/trace", s.TraceID()),
		)
		// Add span ID to the log
		record.AddAttrs(
			slog.Any("logging.googleapis.com/spanId", s.SpanID()),
		)
		// Indicate whether the trace is sampled
		record.AddAttrs(
			slog.Bool("logging.googleapis.com/trace_sampled", s.TraceFlags().IsSampled()),
		)
	}
	// Call the wrapped handler’s Handle method
	return t.Handler.Handle(ctx, record)
}

// Attribute replacer to rename standard log keys for compatibility with Google Cloud Logging
func replacer(groups []string, a slog.Attr) slog.Attr {
	switch a.Key {
	case slog.LevelKey:
		a.Key = "severity" // Rename "level" to "severity" and convert to string format
		if level, ok := a.Value.Any().(slog.Level); ok {
			switch level {
			case LevelDebug:
				a.Value = slog.StringValue("DEBUG")
			case LevelInfo:
				a.Value = slog.StringValue("INFO")
			case LevelNotice:
				a.Value = slog.StringValue("NOTICE")
			case LevelWarning:
				a.Value = slog.StringValue("WARNING")
			case LevelError:
				a.Value = slog.StringValue("ERROR")
			case LevelCritical:
				a.Value = slog.StringValue("CRITICAL")
			case LevelAlert:
				a.Value = slog.StringValue("ALERT")
			case LevelEmergency:
				a.Value = slog.StringValue("EMERGENCY")
			default:
				a.Value = slog.StringValue("DEFAULT")
			}
		}
	case slog.TimeKey:
		a.Key = "timestamp" // Rename "time" to "timestamp"
	case slog.MessageKey:
		a.Key = "messages" // Rename "msg" to "messages"
	}
	return a
}
