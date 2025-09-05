package email

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/smtp"
	"os"
	"strings"
	"time"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cloudevents/sdk-go/v2/event"
)

// Global variables for email configuration
var (
	gmailUser     string
	gmailPassword string
	alertEmail    string
)

// TrendFlag represents the alert data for devices with abnormal trends
type TrendFlag struct {
	DeviceID    string `bigquery:"device_id" json:"device_id"`
	TrendStatus string `bigquery:"trend_status" json:"trend_status"`
	Timestamp1  string `bigquery:"ts_1" json:"ts_1"`
	Timestamp2  string `bigquery:"ts_2" json:"ts_2"`
	Timestamp3  string `bigquery:"ts_3" json:"ts_3"`
}

// MessagePublishedData represents the structure of Pub/Sub CloudEvent messages
type MessagePublishedData struct {
    Message struct {
        Data []byte `json:"data"`
    } `json:"message"`
}

func init() {
	// Load environment variables
	gmailUser = os.Getenv("GMAIL_USER")
	gmailPassword = os.Getenv("GMAIL_APP_PASSWORD")
	alertEmail = os.Getenv("ALERT_EMAIL")
	
	if gmailUser == "" || gmailPassword == "" || alertEmail == "" {
		log.Fatal("Missing required environment variables: GMAIL_USER, GMAIL_APP_PASSWORD, or ALERT_EMAIL")
	}

	log.Printf("Cloud Function inizializzata - Mittente: %s, Destinatario: %s", gmailUser, alertEmail)
	
	// Register the Cloud Function for CloudEvent
	functions.CloudEvent("AlertSubscriber", AlertSubscriber)
}

// AlertSubscriber handles Pub/Sub messages and sends alert emails
func AlertSubscriber(ctx context.Context, e event.Event) error {
	// Set a timeout for the function execution
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Log event information for debugging
	log.Printf("Event received - Type: %s, Source: %s", e.Type(), e.Source())

	// Parse the Pub/Sub message from the CloudEvent
	var msgData  MessagePublishedData
	if err := e.DataAs(&msgData ); err != nil {
		log.Printf("Error parsing Pub/Sub message: %v", err)
		return fmt.Errorf("error parsing Pub/Sub message: %v", err)
	}

	// Check that message data is not empty
	if len(msgData .Message.Data) == 0 {
		log.Printf("Empty message data received")
		return fmt.Errorf("empty message data")
	}

	log.Printf("Message data (length: %d): %s", len(msgData.Message.Data), string(msgData.Message.Data))
	
	// Parse alert data from the message
	var alert TrendFlag
	if err := json.Unmarshal(msgData .Message.Data, &alert); err != nil {
		log.Printf("Error decoding alert data: %v", err)
		return fmt.Errorf("error decoding alert data: %v", err)
	}

	log.Printf("Alert decoded successfully: %+v", alert)

	// Validate required alert fields
	if err := validateAlert(&alert); err != nil {
		log.Printf("Alert validation failed: %v", err)
		return fmt.Errorf("alert validation failed: %v", err)
	}

	// Send the alert email
	if err := sendEmailAlert(ctx, &alert); err != nil {
		log.Printf("Failed to send email alert: %v", err)
		return fmt.Errorf("failed to send email alert: %v", err)
	}

	log.Printf("Email alert sent successfully for device %s", alert.DeviceID)
	return nil
}

func validateAlert(alert *TrendFlag) error {
	if alert.DeviceID == "" {
		return fmt.Errorf("device_id is required")
	}
	if alert.TrendStatus == "" {
		return fmt.Errorf("trend_status is required")
	}
	return nil
}

// sendEmailAlert sends the alert notification email, ctx for future implementation
func sendEmailAlert(ctx context.Context, alert *TrendFlag) error {
	// Build the email subject
	subject := fmt.Sprintf("Device Alert: %s - %s", alert.DeviceID, alert.TrendStatus)
	
	// Build the email body content
	body := buildEmailBody(alert)
	
	// Format the email message with proper headers
	message := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		gmailUser, alertEmail, subject, body)

	// Configure SMTP authentication
	auth := smtp.PlainAuth("", gmailUser, gmailPassword, "smtp.gmail.com")
	
	// Retry logic with exponential backoff
	var err error
	for i := 0; i < 3; i++ {
		err = smtp.SendMail("smtp.gmail.com:587", auth, gmailUser, []string{alertEmail}, []byte(message))
		if err == nil {
			break
		}
		log.Printf("Email send attempt %d failed: %v", i+1, err)
		if i < 2 {
			time.Sleep(time.Second * time.Duration(i+1))
		}
	}

	return err
}

// buildEmailBody constructs the alert email content
func buildEmailBody(alert *TrendFlag) string {
	var body strings.Builder

	body.WriteString("Device Alert Notification\n")
	body.WriteString("============================\n\n")

	body.WriteString("Device ID: " + alert.DeviceID + "\n")
	body.WriteString("Trend Status: " + alert.TrendStatus + "\n")
	body.WriteString("Alert Time: " + time.Now().Format("2006-01-02 15:04:05") + "\n\n")

	body.WriteString("Timestamp Details:\n")
	if alert.Timestamp1 != "" {
		body.WriteString("- Timestamp 1: " + alert.Timestamp1 + "\n")
	}
	if alert.Timestamp2 != "" {
		body.WriteString("- Timestamp 2: " + alert.Timestamp2 + "\n")
	}
	if alert.Timestamp3 != "" {
		body.WriteString("- Timestamp 3: " + alert.Timestamp3 + "\n")
	}

	body.WriteString("\nPlease address this issue as soon as possible.\n")
	body.WriteString("This email was sent automatically. Do not reply.\n")

	return body.String()
}
