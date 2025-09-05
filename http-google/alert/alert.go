package alert

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/pubsub/v2"
	"google.golang.org/api/iterator"
)

var (
	projectID = os.Getenv("GCP_PROJECT")
	topicID   = os.Getenv("PUBSUB_TOPIC")
)

type TrendFlag struct {
	DeviceID    string `bigquery:"device_id" json:"device_id"`
	TrendStatus string `bigquery:"trend_status" json:"trend_status"`
	Timestamp1  string `bigquery:"ts_1" json:"ts_1"`
	Timestamp2  string `bigquery:"ts_2" json:"ts_2"`
	Timestamp3  string `bigquery:"ts_3" json:"ts_3"`
}

func init() {
	if projectID == "" {
		log.Fatal("GCP_PROJECT environment variable is required")
	}
	if topicID == "" {
		log.Fatal("PUBSUB_TOPIC environment variable is required")
	}
}

func AlertHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Minute)
	defer cancel()

	// Create BigQuery client
	bqClient, err := bigquery.NewClient(ctx, projectID)
	if err != nil {
		log.Printf("BigQuery client error: %v", err)
		http.Error(w, "BigQuery client error", http.StatusInternalServerError)
		return
	}
	defer bqClient.Close()

	// Execute query
	query := `
		SELECT device_id, trend_status, 
			FORMAT_TIMESTAMP('%F %T', ts_1) AS ts_1,
			FORMAT_TIMESTAMP('%F %T', ts_2) AS ts_2,
			FORMAT_TIMESTAMP('%F %T', ts_3) AS ts_3
		FROM ` + "`organic-cat-465614-m9.MetricFromClient.trend_flags_table`" + `
		WHERE trend_status = 'UPWARD_TREND'
		LIMIT 1000`

	it, err := bqClient.Query(query).Read(ctx)
	if err != nil {
		log.Printf("BigQuery query error: %v", err)
		http.Error(w, "Query execution error", http.StatusInternalServerError)
		return
	}

	// Read query results
	var alerts []TrendFlag
	for {
		var row TrendFlag
		err := it.Next(&row)
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Printf("Error reading query results: %v", err)
			http.Error(w, "Error reading results", http.StatusInternalServerError)
			return
		}
		alerts = append(alerts, row)
	}

	if len(alerts) == 0 {
		fmt.Fprintln(w, "No anomalies found.")
		return
	}

	// Create Pub/Sub client
	pubClient, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		log.Printf("Pub/Sub client error: %v", err)
		http.Error(w, "Pub/Sub client error", http.StatusInternalServerError)
		return
	}
	defer pubClient.Close()

	publisher := pubClient.Publisher(topicID)
	defer publisher.Stop()

	// Publish messages
	successCount := 0
	for _, alert := range alerts {
		data, err := json.Marshal(alert)
		if err != nil {
			log.Printf("Failed to marshal alert for device %s: %v", alert.DeviceID, err)
			continue
		}

		result := publisher.Publish(ctx, &pubsub.Message{Data: data})
		if _, err := result.Get(ctx); err != nil {
			log.Printf("Failed to publish message for device %s: %v", alert.DeviceID, err)
		} else {
			successCount++
		}
	}

	fmt.Fprintf(w, "Published %d out of %d alerts successfully\n", successCount, len(alerts))
}