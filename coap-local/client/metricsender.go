package main

import (
	"bytes"
	"context"
	//"fmt"
	"github.com/fxamacker/cbor/v2"
	//"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	//"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"gonum.org/v1/gonum/stat/distuv"
	"log"
	"math/rand"
	//"net/http"
	"time"
	"github.com/plgd-dev/go-coap/v3/message"
	"github.com/plgd-dev/go-coap/v3/message/codes"
	"github.com/plgd-dev/go-coap/v3/udp"
	"github.com/plgd-dev/go-coap/v3/udp/client"
)

// Metrics represents the telemetry data collected from a device.
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

// MetricSender simulates a device sending metrics to a remote server.
type MetricSender struct {
	deviceID string
	client   *client.Conn
	tracer   trace.Tracer
	url      string

	// Anomaly simulation
	anomalyStartTime    time.Time
	anomalyDuration     time.Duration
	anomalyHoldDuration time.Duration
	anomalyActive       bool
	baseTemp            float64
}

func NewMetricSender(deviceID, serverAddr, url string, tracer trace.Tracer) *MetricSender {
	c, err := udp.Dial(serverAddr)
	if err != nil {
		log.Fatalf("Failed to create CoAP client for device %s: %v", deviceID, err)
	}
	return &MetricSender{
		deviceID: deviceID,
		client:   c,
		tracer:   tracer,
		url:      url,
	}
}

func (s *MetricSender) SendMetric(ctx context.Context) error {
	maybeTriggerAnomaly(s)

	ctx, span := s.tracer.Start(ctx, "send_metrics",
		trace.WithAttributes(attribute.String("device.id", s.deviceID)))
	defer span.End()

	metric := s.GenerateMetrics()
	data, err := cbor.Marshal(metric)
	if err != nil {
		span.RecordError(err)
		log.Printf("[%s] CBOR marshal error: %v", s.deviceID, err)
		return err
	}

	resp, err := s.client.Post(ctx, s.url, message.AppCBOR, bytes.NewReader(data))
	if err != nil {
		span.RecordError(err)
		log.Printf("[%s] Failed to send metrics: %v", s.deviceID, err)
		return err
	}
	//defer resp.Body().Close()

	if resp.Code() != codes.Created && resp.Code() != codes.Changed {
		log.Printf("[%s] Unexpected response code: %v", s.deviceID, resp.Code())
	} else {
		log.Printf("[%s] Sent metric successfully", s.deviceID)
	}
	return nil
}

// StartAnomaly activates the anomaly simulation for a fixed duration.
func (s *MetricSender) StartAnomaly(duration time.Duration) {
	s.anomalyStartTime = time.Now()
	s.anomalyDuration = duration
	s.anomalyHoldDuration = 3 * time.Minute
	s.anomalyActive = true
	s.baseTemp = 30 + rand.Float64()*35	 // Random base temperature between 30 and 65
}

// maybeTriggerAnomaly probabilistically starts an anomaly based on a normal distribution.
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
		log.Printf("[%s] Triggered anomaly!", s.deviceID)
		s.StartAnomaly(time.Minute * 4)
	}
}

// GenerateMetrics generates realistic metrics, adjusting temperature if anomaly is active.
func (s *MetricSender) GenerateMetrics() Metrics {
	// Distributions for each metric
	cpuDist := distuv.Normal{Mu: 40, Sigma: 10}    
	memDist := distuv.Normal{Mu: 2048, Sigma: 512} 
	normalTempDist := distuv.Normal{Mu: 45, Sigma: 2.5}
	diskUsageDist := distuv.Normal{Mu: 60, Sigma: 20} 
	readDist := distuv.Normal{Mu: 3, Sigma: 1}        
	writeDist := distuv.Normal{Mu: 3, Sigma: 1}

	var temp float64
	if s.anomalyActive {
		elapsed := time.Since(s.anomalyStartTime)
		totalDuration := s.anomalyDuration + s.anomalyHoldDuration

		if elapsed > totalDuration {
			// Anomaly ends
			s.anomalyActive = false
			temp = clamp(normalTempDist.Rand(), 30, 65)
		} else {
			maxTemp := 100.0
			if elapsed <= s.anomalyDuration {
				//  Warming up
				progress := float64(elapsed) / float64(s.anomalyDuration)
				temp = s.baseTemp + progress*(maxTemp-s.baseTemp)
			} else {
				// Holding peak
				temp = maxTemp
			}
		}
	} else {
		temp = clamp(normalTempDist.Rand(), 30, 65)
	}

	return Metrics{
		DeviceID:         s.deviceID,
		Timestamp:        time.Now(),
		CPUPercent:       clamp(cpuDist.Rand(), 0, 100),
		MemUsedMB:        clamp(memDist.Rand(), 0, 4096),
		TempC:            temp,
		DiskUsagePercent: clamp(diskUsageDist.Rand(), 0, 100),
		DiskReadMBps:     clamp(readDist.Rand(), 0, 10),
		DiskWriteMBps:    clamp(writeDist.Rand(), 0, 10),
	}
}


// clamp restricts a float value to the provided min and max bounds.
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