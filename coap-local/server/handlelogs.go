package main

import (
	"github.com/fxamacker/cbor/v2"
	"github.com/plgd-dev/go-coap/v3/message"
	"github.com/plgd-dev/go-coap/v3/message/codes"
	"github.com/plgd-dev/go-coap/v3/mux"
	"go.opentelemetry.io/otel"
	"log"
	"log/slog"
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

// CoAP handler for processing a batch of logs
func handleCoapBatchLog(w mux.ResponseWriter, r *mux.Message) {
	var batch IncomingLogBatch

	// Get the message body
	body, err := r.ReadBody()
	if err != nil {
		log.Printf("Error reading CoAP message body: %v", err)
		w.SetResponse(codes.BadRequest, message.TextPlain, nil)
		return
	}

	// Decode the CBOR-encoded request body into IncomingLogBatch
	if err := cbor.Unmarshal(body, &batch); err != nil {
		log.Printf("Error decoding CBOR: %v", err)
		w.SetResponse(codes.BadRequest, message.TextPlain, nil)
		return
	}

	// Extract tracing context and start a span
	ctx := r.Context()
	ctx, span := otel.Tracer("coap-server").Start(ctx, "handleCoapBatchLog")
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

	// Send CoAP 2.01 Created response to confirm successful processing
	w.SetResponse(codes.Created, message.TextPlain, nil)
}