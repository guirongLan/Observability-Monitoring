package main

import (
	"context"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"log"
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

var (
	meter          metric.Meter
	MCUUsageGauge       metric.Float64ObservableGauge
	MCUTempCGauge      metric.Float64ObservableGauge
	ThermometerCGauge       metric.Float64ObservableGauge
	BarometerHPaGauge metric.Float64ObservableGauge
	HygrometerRHGauge  metric.Float64ObservableGauge
	AnemometerMPSGauge metric.Float64ObservableGauge
)

// initMetrics initializes all the metric instruments (gauges) that will be used
// to observe system metrics like CPU, temperature, memory, and disk usage.
func initMetrics(meter metric.Meter) {
	var err error

	// Create a gauge for MCU usage percentage
	MCUUsageGauge, err = meter.Float64ObservableGauge("custom.googleapis.com/mcu_percent",
		metric.WithDescription("Percentuale di utilizzo della MCU"))
	if err != nil {
		log.Fatalf("failed to create mcu_percent gauge: %v", err)
	}

	// Create a gauge for MCU temperature in Celsius
	MCUTempCGauge, err = meter.Float64ObservableGauge("custom.googleapis.com/mcu_temp_celsius",
		metric.WithDescription("Temperatura della MCU (gradi Celsius)"))
	if err != nil {
		log.Fatalf("failed to create mcu_temp_celsius gauge: %v", err)
	}

	// Create a gauge for external thermometer temperature in Celsius
	ThermometerCGauge, err = meter.Float64ObservableGauge("custom.googleapis.com/external_thermometer_celsius",
		metric.WithDescription("Temperatura esterna (gradi Celsius)"))
	if err != nil {
		log.Fatalf("failed to create external_thermometer_celsius gauge: %v", err)
	}

	// Create a gauge for atmospheric pressure in hPa
	BarometerHPaGauge, err = meter.Float64ObservableGauge("custom.googleapis.com/barometer_hpa",
		metric.WithDescription("Pressione atmosferica (hPa)"))
	if err != nil {
		log.Fatalf("failed to create barometer_hpa gauge: %v", err)
	}

	// Create a gauge for relative humidity percentage
	HygrometerRHGauge, err = meter.Float64ObservableGauge("custom.googleapis.com/hygrometer_rh",
		metric.WithDescription("Umidità relativa (%)"))
	if err != nil {
		log.Fatalf("failed to create hygrometer_rh gauge: %v", err)
	}

	// Create a gauge for wind speed in m/s
	AnemometerMPSGauge, err = meter.Float64ObservableGauge("custom.googleapis.com/anemometer_mps",
		metric.WithDescription("Velocità del vento (m/s)"))
	if err != nil {
		log.Fatalf("failed to create anemometer_mps gauge: %v", err)
	}
}

// registerObservers registers a callback function that OpenTelemetry calls periodically
// to collect the current values for all the defined gauges.
func registerObservers(meter metric.Meter) error {
	_, err := meter.RegisterCallback(
		func(ctx context.Context, observer metric.Observer) error {
			// Lock the cache for safe concurrent access
			cacheMu.RLock()
			defer cacheMu.RUnlock()

			// Iterate over all cached metrics and observe each gauge value with the device ID label
			for _, m := range globalMetricCache {

				labels := metric.WithAttributes(
					attribute.String("device_id", m.DeviceID),
					attribute.Float64("latitude", m.GeoPosition.Latitude),
                    attribute.Float64("longitude", m.GeoPosition.Longitude),
                    attribute.Float64("altitude", m.GeoPosition.Altitude),
					)
				observer.ObserveFloat64(MCUUsageGauge, m.MCUUsagePercent, labels)
				observer.ObserveFloat64(MCUTempCGauge, m.MCUTempC, labels)
				observer.ObserveFloat64(ThermometerCGauge, m.ExternalSensors.ThermometerC, labels)
				observer.ObserveFloat64(BarometerHPaGauge, m.ExternalSensors.BarometerHPa, labels)
				observer.ObserveFloat64(HygrometerRHGauge, m.ExternalSensors.HygrometerRH, labels)
				observer.ObserveFloat64(AnemometerMPSGauge, m.ExternalSensors.AnemometerMPS, labels)

				// Uncomment for debug logging localy:
				// log.Printf("Observed metrics for device %s: CPU %.2f%%, Temp %.2f°C", m.DeviceID, m.CPUPercent, m.TempC)
			}
			return nil
		},
		// List all instruments to be observed in this callback
		MCUUsageGauge, MCUTempCGauge, ThermometerCGauge, BarometerHPaGauge, HygrometerRHGauge, AnemometerMPSGauge,
	)
	return err
}
