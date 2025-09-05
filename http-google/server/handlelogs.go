package main

import (
	"github.com/fxamacker/cbor/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"log"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// IncomingLogBatch represents the structure of a log batch sent by a device
type IncomingLogBatch struct {
	DeviceID string    `cbor:"device_id"`
	Logs     [][]int64 `cbor:"logs"` // Each log is a pair: [event_id, timestamp]
}

// Map of event IDs to their severity and message descriptions
var eventDefinitions = map[uint8]struct {
	Severity string
	Message  string
}{
	1: {"DEBUG", "Dispositivo in fase di inizializzazione"},
	2: {"DEBUG", "Controllo stato rete"},
	3: {"DEBUG", "Avvio modulo sensore"},
	4: {"DEBUG", "Sincronizzazione orologio"},

	5: {"INFO", "Avvio completato"},
	6: {"INFO", "Temperatura normale"},
	7: {"INFO", "CPU sotto soglia"},
	8: {"INFO", "Heartbeat inviato"},

	9:  {"NOTICE", "Cambio configurazione"},
	10: {"NOTICE", "Aggiornamento firmware disponibile"},
	11: {"NOTICE", "Sensore temporaneamente inattivo"},
	12: {"NOTICE", "Collegamento rete ristabilito"},

	13: {"WARNING", "Temperatura elevata"},
	14: {"WARNING", "Consumo CPU sopra la soglia"},
	15: {"WARNING", "Batteria in esaurimento"},
	16: {"WARNING", "Perdita pacchetti rilevata"},

	17: {"ERROR", "Impossibile connettersi al server"},
	18: {"ERROR", "Errore lettura sensore"},
	19: {"ERROR", "Timeout nella risposta del server"},
	20: {"ERROR", "Scrittura su memoria fallita"},

	21: {"CRITICAL", "Perdita connessione permanente"},
	22: {"CRITICAL", "Dati corrotti nella memoria"},

	23: {"ALERT", "Accesso non autorizzato rilevato"},
	24: {"ALERT", "Possibile attacco DoS in corso"},

	25: {"EMERGENCY", "Sistema in stato critico - riavvio necessario"},
	26: {"EMERGENCY", "Errore hardware irreversibile"},
	27: {"EMERGENCY", "Guasto alimentazione principale"},
}

// Maps severity string to slog.Level
func mapSeverityToLevel(sev string) slog.Level {
	switch strings.ToUpper(sev) {
	case "DEBUG":
		return LevelDebug
	case "INFO":
		return LevelInfo
	case "NOTICE":
		return LevelNotice
	case "WARNING":
		return LevelWarning
	case "ERROR":
		return LevelError
	case "CRITICAL":
		return LevelCritical
	case "ALERT":
		return LevelAlert
	case "EMERGENCY":
		return LevelEmergency
	default:
		return LevelInfo
	}
}

// HTTP handler for processing a batch of logs
func handleBatchLog(w http.ResponseWriter, r *http.Request) {
	var batch IncomingLogBatch

	// Decode the CBOR-encoded request body into IncomingLogBatch
	if err := cbor.NewDecoder(r.Body).Decode(&batch); err != nil {
		http.Error(w, "invalid cbor", http.StatusBadRequest)
		return
	}

	// Extract tracing context and start a span
	ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
	ctx, span := otel.Tracer("http-server").Start(ctx, "handleBatchLog")
	defer span.End()

	// Iterate over each compressed log entry
	for _, entry := range batch.Logs {
		// Each entry must be [eventID, timestamp]
		if len(entry) != 2 {
			log.Println("Invalid log entry, skipping:", entry)
			continue
		}

		id := uint8(entry[0])
		ts := entry[1]

		def, ok := eventDefinitions[id]
		if !ok {
			log.Printf("Unknown event ID %d", id)
			continue
		}

		t := time.Unix(ts, 0).UTC()
		formattedTime := t.Format(time.RFC3339)

		// Log the message with context and attributes
		slog.LogAttrs(ctx, mapSeverityToLevel(def.Severity), def.Message,
			slog.String("device_id", batch.DeviceID),
			slog.String("timestamp", formattedTime),
			slog.String("type", "devicelog"),
		)
	}

	// Send HTTP 200 OK to confirm successful processing
	w.WriteHeader(http.StatusOK)
}
