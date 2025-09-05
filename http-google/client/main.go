package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.opentelemetry.io/otel"
)

// Config holds all configuration settings for the system
type Config struct {
	LogURL           string                `json:"log_url"`
	MetricURL        string                `json:"metric_url"`
	BatchSize        int                   `json:"batch_size"`
	BatchInterval    time.Duration         `json:"batch_interval"`
	MetricInterval   time.Duration         `json:"metric_interval"`
	EventGenInterval EventIntervalConfig   `json:"event_gen_interval"`
	DeviceConfigFile string                `json:"device_config_file"`
}

// DevicesConfig represents the structure of the devices configuration file
type DevicesConfig struct {
	Devices []DeviceConfig `json:"devices"`
}

// EventIntervalConfig defines minimum and maximum durations for random event generation
type EventIntervalConfig struct {
    Min time.Duration `json:"min"`
    Max time.Duration `json:"max"`
}

// loadConfig loads the system configuration with default values
func loadConfig() Config {
	cfg := Config{
		LogURL:         "https://http-server-1094805005874.europe-west1.run.app/batchLog",
		MetricURL:      "https://http-server-1094805005874.europe-west1.run.app/batchMetric",
		/* local test
		cfg.LogURL = "http://localhost:8080/batchLog"         // Local testing endpoint
		cfg.MetricURL = "http://localhost:8080/batchMetric"   // Local testing endpoint*/
	
		BatchSize:      30,
		BatchInterval:  5 * time.Minute,
		MetricInterval: 90 * time.Second,
		DeviceConfigFile: "devices.json",
		EventGenInterval: EventIntervalConfig{
			Min: 10 * time.Second,
			Max: 15 * time.Second,
		},
	}
	
	// Try to load configuration from file if it exists
	if configFile := os.Getenv("CONFIG_FILE"); configFile != "" {
		if data, err := os.ReadFile(configFile); err == nil {
			if err := json.Unmarshal(data, &cfg); err != nil {
				log.Printf("Warning: Failed to parse config file %s: %v", configFile, err)
			} else {
				log.Printf("Configuration loaded from %s", configFile)
			}
		}
	}

	log.Printf("Configuration loaded: batch size: %d, metric interval: %v", 
		cfg.BatchSize, cfg.MetricInterval)
	
	return cfg
}

// loadDevicesConfig loads device configurations from external JSON file
func loadDevicesConfig(filename string) ([]DeviceConfig, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read device config file %s: %w", filename, err)
	}

	var devicesConfig DevicesConfig
	if err := json.Unmarshal(data, &devicesConfig); err != nil {
		return nil, fmt.Errorf("failed to parse device config file %s: %w", filename, err)
	}

	return devicesConfig.Devices, nil
}

// newHTTPClient creates an HTTP client with a specified timeout and optimized connection settings
func newHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     100 * time.Second,
		},
	}
}

// handleShutdown handles graceful shutdown on system signals
func handleShutdown(cancelFunc context.CancelFunc) {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	sig := <-signalChan
	log.Printf("Interrupt signal received (%v), shutting down...", sig)
	cancelFunc()
}

func main() {
	log.Println("Starting IoT device simulation system...")

	// Start root context with cancel function
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start a goroutine to handle shutdown signals
	go handleShutdown(cancel)

	// Load main configuration settings
	cfg := loadConfig()

	// Load device configurations from external file
	deviceConfigs, err := loadDevicesConfig(cfg.DeviceConfigFile)
	if err != nil {
		log.Fatalf("Failed to load device configurations: %v", err)
	}

	log.Printf("Loaded %d device configurations from %s", len(deviceConfigs), cfg.DeviceConfigFile)

	// Setup OpenTelemetry tracer
	shutdown, err := setupTracer()
	if err != nil {
		log.Fatalf("Tracer error: %v", err)
	}
	defer shutdown(ctx)

	// Create a tracer instance and HTTP client
	tracer := otel.Tracer("device-simulator")
	client := newHTTPClient(30 * time.Second)

	// Initialize senders for all devices
	logSenders := make([]*LogSender, 0, len(deviceConfigs))
	metricSenders := make([]*MetricSender, 0, len(deviceConfigs))

	for _, deviceConfig := range deviceConfigs {
		// Create log sender for this device
		logSender := NewLogSender(client, tracer, deviceConfig.DeviceID, cfg.LogURL)
		logSenders = append(logSenders, logSender)

		// Create metric sender for this device
		metricSender := NewMetricSender(deviceConfig, client, tracer, cfg.MetricURL)
		metricSenders = append(metricSenders, metricSender)

		log.Printf("Started device: %s at location (%.4f, %.4f, %.0fm)", 
			deviceConfig.DeviceID, 
			deviceConfig.GeoPosition.Latitude, 
			deviceConfig.GeoPosition.Longitude,
			deviceConfig.GeoPosition.Altitude)
	}

	// Start background goroutines
	// Casual events/logs to simulate devices' internal operations
	go runEventGenerators(ctx, logSenders, cfg.EventGenInterval)

	// Send logs periodically in batches
	go runLogSenders(ctx, logSenders, cfg.BatchInterval, cfg.BatchSize)

	// Send metrics periodically
	go runMetricSenders(ctx, metricSenders, cfg.MetricInterval)

	log.Printf("System started with %d devices. Sending metrics every %v", 
		len(deviceConfigs), cfg.MetricInterval)

	// Wait for shutdown signal
	<-ctx.Done()
	log.Println("Shutdown complete")
}