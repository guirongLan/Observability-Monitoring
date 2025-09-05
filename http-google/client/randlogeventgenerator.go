package main

import(
	"context"
	"log"
	"math/rand"
	"time"
)
// runEventGenerators starts a random event generator goroutine for each LogSender
func runEventGenerators(ctx context.Context, senders []*LogSender, intervalRange EventIntervalConfig) {
	for _, sender := range senders {
		go startRandomEventGenerator(ctx, sender, intervalRange)
	}
}

// startRandomEventGenerator starts a random event generator for a single device
func startRandomEventGenerator(ctx context.Context, sender *LogSender, config EventIntervalConfig) {
	// Create a slice containing all available event IDs
	eventIDs := make([]uint8, 0, len(eventDefinitions))
	for id := range eventDefinitions {
		eventIDs = append(eventIDs, id)
	}

	log.Printf("Event generator started for device: %v - Interval range: %v - %v", 
		sender.DeviceID, config.Min, config.Max)

	go func() {
		defer log.Printf("Event generator stopped for device: %v", sender.DeviceID)
		for {
			// Calculate a random interval between min and max durations
			intervalRange := config.Max - config.Min
			randomInterval := config.Min + time.Duration(rand.Int63n(int64(intervalRange)))
			
			select {
			case <-ctx.Done():
				// Stop the generator if context is canceled
				return
			case <-time.After(randomInterval):
				// Generate a random event ID and add it to the sender's log cache
				randomEventID := eventIDs[rand.Intn(len(eventIDs))]
				sender.addEvent(randomEventID)
			}
		}
	}()
}



