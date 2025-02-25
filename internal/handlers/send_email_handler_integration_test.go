package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/smtp"
	"os"
	"testing"

	"github.com/joho/godotenv"
	"github.com/illegalcall/task-master/internal/models"
	"github.com/stretchr/testify/assert"
)

// Load environment variables from .env file before tests run.
func init() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found; relying on environment variables")
	}
}

func TestSendEmailHandler_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Load email configuration from environment variables
	// Optionally, you can uncomment and check for required variables:
	// emailFrom := os.Getenv("EMAIL_FROM")
	// emailPassword := os.Getenv("EMAIL_PASSWORD")
	// emailHost := os.Getenv("EMAIL_HOST")
	// emailPort := os.Getenv("EMAIL_PORT")
	emailRecipient := os.Getenv("EMAIL_RECIPIENT") // Set this in your .env file

	// Check if required environment variables are set; skip if not.
	if emailRecipient == "" {
		t.Skip("Skipping integration test: missing EMAIL_RECIPIENT")
	}

	log.Printf("Testing integration for: %s", emailRecipient)

	// Create a sample email payload using the recipient from env
	payload := models.SendEmailPayload{
		Recipient:   emailRecipient,
		Subject:     "Integration Test Email",
		Body:        "This is a test email sent from the integration test.",
		Attachments: []string{}, // You can add a real file path here if desired
	}
	payloadBytes, _ := json.Marshal(payload)

	// Call the SendEmailHandler
	result, err := SendEmailHandler(payloadBytes)

	// Assertions
	assert.NoError(t, err)
	assert.Equal(t, "Email sent successfully", result.Message)

	// Optional: Check the inbox of the test email account manually or via an API.
	log.Println("Please check the inbox of", emailRecipient, "to verify the email was received.")
}

// Helper function to send a test email directly using smtp (for debugging purposes)
func sendTestEmail(from, password, host, port, recipient, subject, body string) error {
	// Set up authentication information.
	auth := smtp.PlainAuth("", from, password, host)

	// Connect to the server, authenticate, set the sender and recipient, and send the email.
	addr := fmt.Sprintf("%s:%s", host, port)
	msg := []byte("To: " + recipient + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"\r\n" +
		body + "\r\n")

	return smtp.SendMail(addr, auth, from, []string{recipient}, msg)
}
