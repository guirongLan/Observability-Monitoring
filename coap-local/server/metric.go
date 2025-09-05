package main

import (
	"context"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"log"
)

var (
	meter          metric.Meter
	cpuGauge       metric.Float64ObservableGauge
	tempGauge      metric.Float64ObservableGauge
	memGauge       metric.Float64ObservableGauge
	diskUsageGauge metric.Float64ObservableGauge
	diskReadGauge  metric.Float64ObservableGauge
	diskWriteGauge metric.Float64ObservableGauge
)

// initMetrics initializes all the metric instruments (gauges) that will be used
// to observe system metrics like CPU, temperature, memory, and disk usage.
func initMetrics(meter metric.Meter) {
	var err error

	// Create a gauge for CPU usage percentage
	cpuGauge, err = meter.Float64ObservableGauge("custom.googleapis.com/cpu_percent",
		metric.WithDescription("Percentuale di utilizzo della CPU"))
	if err != nil {
		log.Fatalf("failed to create cpu_percent gauge: %v", err)
	}

	// Create a gauge for temperature in Celsius
	tempGauge, err = meter.Float64ObservableGauge("custom.googleapis.com/temperature_celsius",
		metric.WithDescription("Temperatura (gradi Celsius)"))
	if err != nil {
		log.Fatalf("failed to create temperature gauge: %v", err)
	}

	// Create a gauge for memory used in megabytes
	memGauge, err = meter.Float64ObservableGauge("custom.googleapis.com/memory_used_mb",
		metric.WithDescription("Memoria utilizzata (MB)"))
	if err != nil {
		log.Fatalf("failed to create memory_used_mb gauge: %v", err)
	}

	// Create a gauge for disk usage percentage
	diskUsageGauge, err = meter.Float64ObservableGauge("custom.googleapis.com/disk_usage_percent",
		metric.WithDescription("Percentuale di utilizzo del disco"))
	if err != nil {
		log.Fatalf("failed to create disk_usage_percent gauge: %v", err)
	}

	// Create a gauge for disk read speed in MB/s
	diskReadGauge, err = meter.Float64ObservableGauge("custom.googleapis.com/disk_read_mbps",
		metric.WithDescription("Velocità di lettura del disco (MB/s)"))
	if err != nil {
		log.Fatalf("failed to create disk_read_mbps gauge: %v", err)
	}

	// Create a gauge for disk write speed in MB/s
	diskWriteGauge, err = meter.Float64ObservableGauge("custom.googleapis.com/disk_write_mbps",
		metric.WithDescription("Velocità di scrittura del disco (MB/s)"))
	if err != nil {
		log.Fatalf("failed to create disk_write_mbps gauge: %v", err)
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
				labels := metric.WithAttributes(attribute.String("device_id", m.DeviceID))
				observer.ObserveFloat64(cpuGauge, m.CPUPercent, labels)
				observer.ObserveFloat64(tempGauge, m.TempC, labels)
				observer.ObserveFloat64(memGauge, m.MemUsedMB, labels)
				observer.ObserveFloat64(diskUsageGauge, m.DiskUsagePercent, labels)
				observer.ObserveFloat64(diskReadGauge, m.DiskReadMBps, labels)
				observer.ObserveFloat64(diskWriteGauge, m.DiskWriteMBps, labels)

				// Uncomment for debug logging localy:
				// log.Printf("Observed metrics for device %s: CPU %.2f%%, Temp %.2f°C", m.DeviceID, m.CPUPercent, m.TempC)
			}
			return nil
		},
		// List all instruments to be observed in this callback
		cpuGauge, tempGauge, memGauge, diskUsageGauge, diskReadGauge, diskWriteGauge,
	)
	return err
}
