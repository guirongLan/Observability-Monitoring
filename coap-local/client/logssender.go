package main

import (
	"bytes"
	"context"
	cbor "github.com/fxamacker/cbor/v2"
	"go.opentelemetry.io/otel/trace"
	"log"

	"sync"
	"time"
	"github.com/plgd-dev/go-coap/v3/udp"
	"github.com/plgd-dev/go-coap/v3/udp/client"
	"github.com/plgd-dev/go-coap/v3/message"
	"github.com/plgd-dev/go-coap/v3/message/codes"
)

// definizione di vari id che serve alla parte server
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

type LogEntryCompact [2]int64

// LogSender represents a device that sends randomly generated logs
type LogSender struct {
	client   *client.Conn
	tracer     trace.Tracer
	deviceID   string
	url        string
	logCache   []LogEntryCompact
	cacheMutex sync.Mutex
}

// NewLogSender creates a new LogSender with its own CoAP client
func NewLogSender(deviceID, serverAddr, url string, tracer trace.Tracer) *LogSender {
	c, err := udp.Dial(serverAddr)
	if err != nil {
		log.Fatalf("Failed to create CoAP client for device %s: %v", deviceID, err)
	}
	return &LogSender{
		client:   c,
		tracer:   tracer,
		deviceID: deviceID,
		url:      url,
	}
}

// Send sends a batch of log entries to the configured URL using CBOR encoding and OpenTelemetry tracing
func (s *LogSender) Send(ctx context.Context, entries []LogEntryCompact) error {
	if len(entries) == 0 {
		return nil
	}

	ctx, span := s.tracer.Start(ctx, "send_log_batch")
	defer span.End()

	payload := map[string]interface{}{
		"device_id": s.deviceID,
		"logs":      entries,
	}

	data, err := cbor.Marshal(payload)
	if err != nil {
		span.RecordError(err)
		return err
	}

	resp, err := s.client.Post(ctx, s.url, message.AppCBOR, bytes.NewReader(data))
	if err != nil {
		span.RecordError(err)
		log.Printf("[%s] Failed to send logs: %v", s.deviceID, err)
		return err
	}
	//defer resp.Body().Close()

	if resp.Code() != codes.Created && resp.Code() != codes.Changed {
		log.Printf("[%s] Unexpected response code: %v", s.deviceID, resp.Code())
	} else {
		log.Printf("[%s] Sent %d logs successfully", s.deviceID, len(entries))
	}
	return nil
}

// addEvent adds a new event with the given ID to the log cache
func (s *LogSender) addEvent(id uint8) {
	// Check if the event ID is defined
	if _, ok := eventDefinitions[id]; !ok {
		log.Printf("Undefined event ID: %d", id)
		return
	}
	ts := time.Now().Unix()
	// Append the event ID and timestamp to the log cache
	s.AddLog(LogEntryCompact{int64(id), ts})
	log.Printf("Device %s generated event ID: %d", s.deviceID, id)
}

// AddLog safely appends a log entry to the cache with mutex locking
func (s *LogSender) AddLog(entry LogEntryCompact) {
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()
	// Append entry to the cache
	s.logCache = append(s.logCache, entry)

	// Limit cache size to last 200 entries to avoid unbounded growth
	if len(s.logCache) > 200 {
    s.logCache = s.logCache[len(s.logCache)-200:]
}
}
// SendBatch copies a batch of logs from cache and sends them without holding the lock during send
func (s *LogSender) SendBatch(ctx context.Context, batchSize int) error {
    s.cacheMutex.Lock()
    if len(s.logCache) == 0 {
        s.cacheMutex.Unlock()
        return nil
    }

    var entries []LogEntryCompact
    if len(s.logCache) > batchSize {
        entries = make([]LogEntryCompact, batchSize)
        copy(entries, s.logCache[:batchSize])
        s.logCache = s.logCache[batchSize:]
    } else {
        entries = s.logCache
        s.logCache = nil
    }
    s.cacheMutex.Unlock()

   	// Send logs without holding the mutex lock
    return s.Send(ctx, entries)
}

// runLogSenders runs a loop that periodically sends batches of logs for all devices until context is cancelled
func runLogSenders(ctx context.Context, senders []*LogSender, interval time.Duration, batchSize int) {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            log.Println("Stopping log senders...")
            return
        case <-ticker.C:
            for _, sender := range senders {
                if err := sender.SendBatch(ctx, batchSize); err != nil {
                    log.Printf("[Device %s] Error sending logs: %v", sender.deviceID, err)
                }
            }
        }
    }
}