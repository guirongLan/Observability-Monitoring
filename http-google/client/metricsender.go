package main

import (
	"bytes"
	"context"
	"fmt"
	"github.com/fxamacker/cbor/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"gonum.org/v1/gonum/stat/distuv"
	"log"
	//"math/rand"
	"net/http"
	"time"
)
// GeoPosition represents the geographical coordinates of a device
type GeoPosition struct {
	Latitude  float64 `cbor:"latitude" json:"latitude"`
	Longitude float64 `cbor:"longitude" json:"longitude"`
	Altitude  float64 `cbor:"altitude" json:"altitude"` // meters above sea level
}

// ExternalSensors represents data from external sensors
type ExternalSensors struct {
	ThermometerC  float64 `cbor:"thermometer_c" json:"thermometer_c"`     // External temperature in Celsius
	BarometerHPa  float64 `cbor:"barometer_hpa" json:"barometer_hpa"`     // Atmospheric pressure in hPa
	HygrometerRH  float64 `cbor:"hygrometer_rh" json:"hygrometer_rh"`     // Relative humidity percentage
	AnemometerMPS float64 `cbor:"anemometer_mps" json:"anemometer_mps"`   // Wind speed in m/s
}

// Metrics represents the telemetry data collected from a device
type Metrics struct {
	DeviceID         string          `cbor:"device_id" json:"device_id"`
	GeoPosition      GeoPosition     `cbor:"geo_position" json:"geo_position"`
	Timestamp        time.Time       `cbor:"timestamp" json:"timestamp"`
	MCUUsagePercent  float64         `cbor:"mcu_usage_percent" json:"mcu_usage_percent"`
	MCUTempC         float64         `cbor:"mcu_temp_c" json:"mcu_temp_c"`
	ExternalSensors  ExternalSensors `cbor:"external_sensors" json:"external_sensors"`
}

// DeviceConfig represents the configuration for a single device
type DeviceConfig struct {
	DeviceID    string      `json:"device_id"`
	GeoPosition GeoPosition `json:"geo_position"`
	// Base values for sensor simulation
	BaseMCUTemp      float64 `json:"base_mcu_temp"`
	BaseThermometer  float64 `json:"base_thermometer"`
	BaseBarometer    float64 `json:"base_barometer"`
	BaseHygrometer   float64 `json:"base_hygrometer"`
	BaseAnemometer   float64 `json:"base_anemometer"`
}

// MetricSender simulates a device sending metrics to a remote server
type MetricSender struct {
	Config   DeviceConfig
	Client   *http.Client
	Tracer   trace.Tracer
	URL      string

	// Anomaly simulation
	anomalyStartTime    time.Time
	anomalyDuration     time.Duration
	anomalyHoldDuration time.Duration
	anomalyActive       bool
}

// NewMetricSender creates and returns a new MetricSender instance
func NewMetricSender(config DeviceConfig, client *http.Client, tracer trace.Tracer, url string) *MetricSender {
	return &MetricSender{
		Config: config,
		Client: client,
		Tracer: tracer,
		URL:    url,
	}
}

// StartAnomaly activates the anomaly simulation for a fixed duration
func (s *MetricSender) StartAnomaly(duration time.Duration) {
	s.anomalyStartTime = time.Now()
	s.anomalyDuration = duration
	s.anomalyHoldDuration = 3 * time.Minute
	s.anomalyActive = true
}

// maybeTriggerAnomaly probabilistically starts an anomaly based on a normal distribution
func maybeTriggerAnomaly(s *MetricSender) {
	if s.anomalyActive {
		return
	}

	normal := distuv.Normal{
		Mu:    0,
		Sigma: 1,
	}
	z := normal.Rand()

	if z > 2.0 { // ~2.2% chance of triggering
		log.Printf("[%s] Triggered anomaly!", s.Config.DeviceID)
		s.StartAnomaly(time.Minute * 4)
	}
}

// GenerateMetrics generates realistic metrics with external sensors
func (s *MetricSender) GenerateMetrics() Metrics {
	// Distributions for each metric
	mcuUsageDist := distuv.Normal{Mu: 45, Sigma: 15}
	
	// MCU temperature - can be affected by anomalies
	var mcuTemp float64
	if s.anomalyActive {
		elapsed := time.Since(s.anomalyStartTime)
		totalDuration := s.anomalyDuration + s.anomalyHoldDuration

		if elapsed > totalDuration {
			// Anomaly ends
			s.anomalyActive = false
			normalMCUTempDist := distuv.Normal{Mu: s.Config.BaseMCUTemp, Sigma: 3}
			mcuTemp = clamp(normalMCUTempDist.Rand(), 20, 70)
		} else {
			maxTemp := 100.0
			if elapsed <= s.anomalyDuration {
				// Warming up
				progress := float64(elapsed) / float64(s.anomalyDuration)
				mcuTemp = s.Config.BaseMCUTemp + progress*(maxTemp-s.Config.BaseMCUTemp)
			} else {
				// Holding peak
				mcuTemp = maxTemp
			}
		}
	} else {
		normalMCUTempDist := distuv.Normal{Mu: s.Config.BaseMCUTemp, Sigma: 3}
		mcuTemp = clamp(normalMCUTempDist.Rand(), 20, 70)
	}

	// External sensors - simulate environmental variations
	thermometerDist := distuv.Normal{Mu: s.Config.BaseThermometer, Sigma: 2}
	barometerDist := distuv.Normal{Mu: s.Config.BaseBarometer, Sigma: 5}
	hygrometerDist := distuv.Normal{Mu: s.Config.BaseHygrometer, Sigma: 8}
	anemometerDist := distuv.Normal{Mu: s.Config.BaseAnemometer, Sigma: 1.5}

	return Metrics{
		DeviceID:    s.Config.DeviceID,
		GeoPosition: s.Config.GeoPosition,
		Timestamp:   time.Now(),
		MCUUsagePercent: clamp(mcuUsageDist.Rand(), 0, 100),
		MCUTempC:        mcuTemp,
		ExternalSensors: ExternalSensors{
			ThermometerC:  clamp(thermometerDist.Rand(), -40, 60),
			BarometerHPa:  clamp(barometerDist.Rand(), 950, 1050),
			HygrometerRH:  clamp(hygrometerDist.Rand(), 10, 100),
			AnemometerMPS: clamp(anemometerDist.Rand(), 0, 25),
		},
	}
}

// SendMetric sends the generated metrics to the configured HTTP endpoint
func (s *MetricSender) SendMetric(ctx context.Context) error {
	maybeTriggerAnomaly(s)

	ctx, span := s.Tracer.Start(ctx, "SendMetric",
		trace.WithAttributes(attribute.String("device.id", s.Config.DeviceID)))
	defer span.End()

	metric := s.GenerateMetrics()

	// Print locally
	fmt.Printf("[%s] Sending metric: MCU: %.1f%% %.1fC, Ext: %.1fC %.1fhPa %.1f%% %.1fm/s\n", 
		s.Config.DeviceID,
		metric.MCUUsagePercent, metric.MCUTempC,
		metric.ExternalSensors.ThermometerC, metric.ExternalSensors.BarometerHPa,
		metric.ExternalSensors.HygrometerRH, metric.ExternalSensors.AnemometerMPS)

	// Encode to CBOR
	payload, err := cbor.Marshal(metric)
	if err != nil {
		log.Printf("[%s] CBOR marshal error: %v", s.Config.DeviceID, err)
		return err
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.URL, bytes.NewReader(payload))
	if err != nil {
		log.Printf("[%s] Request build error: %v", s.Config.DeviceID, err)
		return err
	}
	req.Header.Set("Content-Type", "application/cbor")

	// Inject trace context into HTTP headers
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))
	
	// Perform request
	resp, err := s.Client.Do(req)
	if err != nil {
		log.Printf("[%s] Send error: %v", s.Config.DeviceID, err)
		return err
	}
	defer resp.Body.Close()

	log.Printf("[%s] Metric sent, status: %s", s.Config.DeviceID, resp.Status)
	return nil
}

// clamp restricts a float value to the provided min and max bounds
func clamp(val, min, max float64) float64 {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

// runMetricSenders starts all metric senders on a fixed interval.
func runMetricSenders(ctx context.Context, senders []*MetricSender, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Stopping metric senders...")
			return
		case <-ticker.C:
			// creo tutti metric sender necessari
			for _, sender := range senders {
				go sender.SendMetric(ctx)
			}
		}
	}
}