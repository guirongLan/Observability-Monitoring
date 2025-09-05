package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/opensearch-project/opensearch-go"
	"github.com/opensearch-project/opensearch-go/opensearchapi"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

var (
	projectID       = "organic-cat-465614-m9"     
	datasetID       = "MetricFromClient"  
	tableID         = "run_googleapis_com_stdout"
	credentialsFile = "C:\\Users\\langu\\Desktop\\distributed-observability\\http-google\\bigqueryOpensearchSync\\organic-cat-465614-m9-6f2aef9852c2.json"
)

// check env
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

// Config 
type Config struct {
	
	BigQuery struct {
		ProjectID string `json:"project_id"`
		Dataset   string `json:"dataset"`
		Table     string `json:"table"`
	} `json:"bigquery"`

	OpenSearch struct {
		URLs     []string `json:"urls"`
		Username string   `json:"username,omitempty"`
		Password string   `json:"password,omitempty"`
		Index    string   `json:"index"`
	} `json:"opensearch"`

	SyncInterval time.Duration `json:"sync_interval"`
}

// LogEntry 
type LogEntry struct {
	LogName           string    `bigquery:"logName" json:"logName"`
	ResourceType      string    `bigquery:"resource_type" json:"resource_type"`
	RevisionName      string    `bigquery:"revision_name" json:"revision_name"`
	Location          string    `bigquery:"location" json:"location"`
	ProjectID         string    `bigquery:"project_id" json:"project_id"`
	ConfigurationName string    `bigquery:"configuration_name" json:"configuration_name"`
	ServiceName       string    `bigquery:"service_name" json:"service_name"`
	JSONPayloadValue  float32 	`bigquery:"jsonPayload_value" json:"jsonPayload_value"`
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

// SyncService 
type SyncService struct {
	config     *Config
	bqClient   *bigquery.Client
	osClient   *opensearch.Client
	lastSync   time.Time
}

// NewSyncService 
func NewSyncService(config *Config) (*SyncService, error) {
	ctx := context.Background()
	
	// inti BigQuery client- with specift auth doc
	var bqClient *bigquery.Client
	var err error
	
	if credentialsFile != "" {
		bqClient, err = bigquery.NewClient(ctx, projectID, option.WithCredentialsFile(credentialsFile))
	} else {
		bqClient, err = bigquery.NewClient(ctx, projectID)
	}
	
	if err != nil {
		return nil, fmt.Errorf("failed to create BigQuery client: %v", err)
	}

	// init OpenSearch client
	osConfig := opensearch.Config{
		Addresses: config.OpenSearch.URLs,
	}
	
	if config.OpenSearch.Username != "" && config.OpenSearch.Password != "" {
		osConfig.Username = config.OpenSearch.Username
		osConfig.Password = config.OpenSearch.Password
	}

	osClient, err := opensearch.NewClient(osConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenSearch client: %v", err)
	}

	return &SyncService{
		config:     config,
		bqClient:   bqClient,
		osClient:   osClient,
		lastSync:   time.Now().Add(-config.SyncInterval),
	}, nil
}

// fetchLogsFromBigQuery 
func (s *SyncService) fetchLogsFromBigQuery(ctx context.Context, since time.Time) ([]*LogEntry, error) {
	query := s.bqClient.Query(fmt.Sprintf(`
		SELECT
  		  logName,
  		  resource.type AS resource_type,
  		  resource.labels.revision_name,
  		  resource.labels.location,
  		  resource.labels.project_id,
  		  resource.labels.configuration_name,
  		  resource.labels.service_name,
		  jsonPayload.value AS jsonPayload_value,
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
		FROM `+"`%s.%s.%s`"+`
		WHERE timestamp >= @since_time
		ORDER BY timestamp ASC
	`, projectID, datasetID, tableID))

	query.Parameters = []bigquery.QueryParameter{
		{
			Name:  "since_time",
			Value: since,
		},
	}

	it, err := query.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to execute BigQuery query: %v", err)
	}

	var logs []*LogEntry
	for {
		var log LogEntry
		err := it.Next(&log)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read query results: %v", err)
		}
		logs = append(logs, &log)
	}

	return logs, nil
}

// sendToOpenSearch send data to OpenSearch
func (s *SyncService) sendToOpenSearch(ctx context.Context, logs []*LogEntry) error {
	if len(logs) == 0 {
		log.Println("No new logs to sync")
		return nil
	}

	// batch
	var bulkBody strings.Builder
	//faccendo come sotto si crea ad ogni giorno una nuova index
	//indexName := fmt.Sprintf("%s-%s", s.config.OpenSearch.Index, time.Now().Format("2006-01-02"))
	indexName := s.config.OpenSearch.Index

	for _, logEntry := range logs {
		// index
		indexOp := map[string]interface{}{
			"index": map[string]interface{}{
				"_index": indexName,
			},
		}
		
		indexOpJSON, err := json.Marshal(indexOp)
		if err != nil {
			return fmt.Errorf("failed to marshal index operation: %v", err)
		}
		
		bulkBody.WriteString(string(indexOpJSON))
		bulkBody.WriteString("\n")
		
		// doc data
		docJSON, err := json.Marshal(logEntry)
		if err != nil {
			return fmt.Errorf("failed to marshal log entry: %v", err)
		}
		
		bulkBody.WriteString(string(docJSON))
		bulkBody.WriteString("\n")
	}

	// batch insert
	req := opensearchapi.BulkRequest{
		Body: strings.NewReader(bulkBody.String()),
	}

	res, err := req.Do(ctx, s.osClient)
	if err != nil {
		return fmt.Errorf("failed to execute bulk request: %v", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("bulk request failed with status: %s", res.Status())
	}

	log.Printf("Successfully indexed %d documents to OpenSearch", len(logs))
	return nil
}

// createIndexTemplate 
func (s *SyncService) createIndexTemplate(ctx context.Context) error {
	templateName := s.config.OpenSearch.Index + "_template"
	
	template := map[string]interface{}{
		"index_patterns": []string{s.config.OpenSearch.Index + "-*"},
		"template": map[string]interface{}{
			"mappings": map[string]interface{}{
				"properties": map[string]interface{}{
					"logName": map[string]interface{}{
						"type": "keyword",
					},
					"resource_type": map[string]interface{}{
						"type": "keyword",
					},
					"revision_name": map[string]interface{}{
						"type": "keyword",
					},
					"location": map[string]interface{}{
						"type": "keyword",
					},
					"project_id": map[string]interface{}{
						"type": "keyword",
					},
					"configuration_name": map[string]interface{}{
						"type": "keyword",
					},
					"service_name": map[string]interface{}{
						"type": "keyword",
					},
					"jsonPayload_value": map[string]interface{}{
						"type": "keyword",
					},
					"jsonPayload_type": map[string]interface{}{
						"type": "keyword",
					},
					"message": map[string]interface{}{
						"type": "text",
						"analyzer": "standard",
					},
					"device_id": map[string]interface{}{
						"type": "keyword",
					},
					"log_timestamp": map[string]interface{}{
						"type": "keyword",
					},
					"timestamp": map[string]interface{}{
						"type": "date",
					},
					"receiveTimestamp": map[string]interface{}{
						"type": "date",
					},
					"severity": map[string]interface{}{
						"type": "keyword",
					},
					"insertId": map[string]interface{}{
						"type": "keyword",
					},
					"instanceid": map[string]interface{}{
						"type": "keyword",
					},
					"trace": map[string]interface{}{
						"type": "keyword",
					},
					"spanId": map[string]interface{}{
						"type": "keyword",
					},
				},
			},
			"settings": map[string]interface{}{
				"number_of_shards":   1,
				"number_of_replicas": 0,
			},
		},
	}

	templateJSON, err := json.Marshal(template)
	if err != nil {
		return fmt.Errorf("failed to marshal index template: %v", err)
	}

	req := opensearchapi.IndicesPutIndexTemplateRequest{
		Name: templateName,
		Body: strings.NewReader(string(templateJSON)),
	}

	res, err := req.Do(ctx, s.osClient)
	if err != nil {
		return fmt.Errorf("failed to create index template: %v", err)
	}
	defer res.Body.Close()

	if res.IsError() && res.StatusCode != 400 { // 400 means template already exists
		return fmt.Errorf("failed to create index template: %s", res.Status())
	}

	log.Printf("Index template '%s' created successfully", templateName)
	return nil
}

// syncOnce 
func (s *SyncService) syncOnce(ctx context.Context) error {
	start := time.Now()
	
	// get BigQuery new data
	logs, err := s.fetchLogsFromBigQuery(ctx, s.lastSync)
	if err != nil {
		return fmt.Errorf("failed to fetch logs from BigQuery: %v", err)
	}

	log.Printf("Fetched %d logs from BigQuery", len(logs))

	// send to OpenSearch
	if err := s.sendToOpenSearch(ctx, logs); err != nil {
		return fmt.Errorf("failed to send logs to OpenSearch: %v", err)
	}

	// update time
	s.lastSync = start
	
	log.Printf("Sync completed in %v", time.Since(start))
	return nil
}

// Start sync
func (s *SyncService) Start(ctx context.Context) error {
	// create index
	if err := s.createIndexTemplate(ctx); err != nil {
		log.Printf("Warning: failed to create index template: %v", err)
	}

	// init
	log.Println("Starting initial sync...")
	if err := s.syncOnce(ctx); err != nil {
		log.Printf("Initial sync failed: %v", err)
	}

	// ticker sync
	ticker := time.NewTicker(s.config.SyncInterval)
	defer ticker.Stop()

	log.Printf("Starting periodic sync every %v", s.config.SyncInterval)
	
	for {
		select {
		case <-ctx.Done():
			log.Println("Sync service stopped")
			return ctx.Err()
		case <-ticker.C:
			if err := s.syncOnce(ctx); err != nil {
				log.Printf("Sync failed: %v", err)
				// 可以添加重试逻辑或报警
			}
		}
	}
}

// Close client
func (s *SyncService) Close() error {
	return s.bqClient.Close()
}

func main() {
	// check env
	checkEnv()
	
	// config
	config := &Config{
		SyncInterval: 5 * time.Minute,
	}
	
	config.BigQuery.ProjectID = projectID
	config.BigQuery.Dataset = datasetID
	config.BigQuery.Table = tableID
	
	// OpenSearch config 
	config.OpenSearch.URLs = []string{"http://localhost:9200"}
	config.OpenSearch.Index = "gcp-logs-table"

	// config.OpenSearch.Username = "admin"
	// config.OpenSearch.Password = "password"

	log.Printf("Starting BigQuery to OpenSearch sync service")
	log.Printf("Project: %s", projectID)
	log.Printf("Dataset: %s", datasetID) 
	log.Printf("Table: %s", tableID)
	log.Printf("OpenSearch: %v", config.OpenSearch.URLs)
	log.Printf("Sync interval: %v", config.SyncInterval)

	// create sync service
	service, err := NewSyncService(config)
	if err != nil {
		log.Fatalf("Failed to create sync service: %v", err)
	}
	defer service.Close()

	// start sync
	ctx := context.Background()
	if err := service.Start(ctx); err != nil {
		log.Fatalf("Sync service failed: %v", err)
	}
}