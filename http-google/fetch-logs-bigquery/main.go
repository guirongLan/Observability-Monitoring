package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"cloud.google.com/go/bigquery"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

var (
	projectID       = "organic-cat-465614-m9"     
	datasetID       = "Logs_Opensearch_BigQuery"  
	tableID         = "run_googleapis_com_stdout" 
	credentialsFile = "C:\\Users\\langu\\Desktop\\distributed-observability\\http-google\\fetch-logs-bigquery\\organic-cat-465614-m9-6f2aef9852c2.json"
)

type LogEntry struct {
	LogName           string    `bigquery:"logName" json:"logName"`
	ResourceType      string    `bigquery:"resource_type" json:"resource_type"`
	RevisionName      string    `bigquery:"revision_name" json:"revision_name"`
	Location          string    `bigquery:"location" json:"location"`
	ProjectID         string    `bigquery:"project_id" json:"project_id"`
	ConfigurationName string    `bigquery:"configuration_name" json:"configuration_name"`
	ServiceName       string    `bigquery:"service_name" json:"service_name"`
	JSONPayloadType   string    `bigquery:"jsonPayload_type" json:"jsonPayload_type"`
	Message           string    `bigquery:"message" json:"message"`
	DeviceID          string    `bigquery:"device_id" json:"device_id"`
	LogTimestamp      string    `bigquery:"log_timestamp" json:"log_timestamp"`
	Timestamp         time.Time `bigquery:"timestamp" json:"timestamp"`
	ReceiveTimestamp  time.Time `bigquery:"receiveTimestamp" json:"receiveTimestamp"`
	Severity          string    `bigquery:"severity" json:"severity"`
	InsertID          string    `bigquery:"insertId" json:"insertId"`
	InstanceID        string    `bigquery:"instanceid" json:"instanceid"`
	Trace             string    `bigquery:"trace" json:"trace"`
	SpanID            string    `bigquery:"spanId" json:"spanId"`
}

func main() {
	ctx := context.Background()

	
	checkEnv()

	client, err := bigquery.NewClient(ctx, projectID, option.WithCredentialsFile(credentialsFile))

	if err != nil {
		log.Fatalf("Failed to create BigQuery client: %v", err)
	}
	defer client.Close()

	log.Println("Running query for logs from last 24 hours...")

	queryString := buildQuery()
	q := client.Query(queryString)

	it, err := q.Read(ctx)
	if err != nil {
		log.Fatalf("Failed to run query: %v", err)
	}

	var results []LogEntry
	for {
		var row LogEntry
		err := it.Next(&row)
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Fatalf("Error reading row: %v", err)
		}
		results = append(results, row)
	}

	if len(results) == 0 {
		log.Println("No logs found in the last 24 hours.")
		return
	}

	if err := saveAsJSON(results); err != nil {
		log.Fatalf("Failed to save results: %v", err)
	}

	log.Printf("Saved %d log entries to file.\n", len(results))
}

func buildQuery() string {
	return fmt.Sprintf(`
SELECT
  logName,
  resource.type AS resource_type,
  resource.labels.revision_name,
  resource.labels.location,
  resource.labels.project_id,
  resource.labels.configuration_name,
  resource.labels.service_name,
  jsonPayload.type AS jsonPayload_type,
  jsonPayload.messages AS message,
  jsonPayload.device_id AS device_id,
  jsonPayload.timestamp AS log_timestamp,
  timestamp,
  receiveTimestamp,
  severity,
  insertId,
  labels.instanceid,
  trace,
  spanId
FROM
  `+"`%s.%s.%s`"+`
WHERE
  timestamp >= TIMESTAMP_SUB(CURRENT_TIMESTAMP(), INTERVAL 24 HOUR)
ORDER BY
  timestamp ASC
LIMIT 5000
`, projectID, datasetID, tableID)
}

func saveAsJSON(data []LogEntry) error {
	timestamp := time.Now().Format("2006-01-02_150405")
	filename := fmt.Sprintf("logs_%s.json", timestamp)
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("create file failed: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("JSON encode failed: %w", err)
	}

	log.Printf("Logs written to: %s\n", filename)
	return nil
}

// 检查环境变量是否设置
func checkEnv() {
	missing := false
	if projectID == "" {
		log.Println("Missing env var: GCP_PROJECT")
		missing = true
	}
	if datasetID == "" {
		log.Println("Missing env var: DATASET_ID")
		missing = true
	}
	if tableID == "" {
		log.Println("Missing env var: TABLE_ID")
		missing = true
	}
	if missing {
		log.Fatal("Required environment variables are missing.")
	}
}
