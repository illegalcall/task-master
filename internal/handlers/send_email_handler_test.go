package handlers

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/illegalcall/task-master/internal/models"
	"github.com/stretchr/testify/assert"
)

// -----------------------------------------------------------------------------
// Mock SMTP Server for Local Testing
// -----------------------------------------------------------------------------

type mockSMTPServer struct {
	addr     string
	messages []string
	listener net.Listener
}

func newMockSMTPServer() *mockSMTPServer {
	return &mockSMTPServer{
		addr: "localhost:2525", // Use a non-standard port to avoid conflicts
	}
}

func (s *mockSMTPServer) start() error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	s.listener = ln
	go s.listenAndServe()
	return nil
}

func (s *mockSMTPServer) listenAndServe() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			break
		}
		go s.handleConnection(conn)
	}
}

func (s *mockSMTPServer) handleConnection(conn net.Conn) {
	defer conn.Close()

	// Send initial 220 greeting.
	conn.Write([]byte("220 mock.smtp.server Service Ready\r\n"))

	scanner := bufio.NewScanner(conn)
	var builder strings.Builder
	quit := false

	for scanner.Scan() {
		line := scanner.Text()
		builder.WriteString(line + "\n")
		switch {
		case strings.HasPrefix(line, "EHLO"), strings.HasPrefix(line, "HELO"):
			conn.Write([]byte("250-mock.smtp.server Hello\r\n250 AUTH LOGIN PLAIN\r\n"))
		case strings.HasPrefix(line, "AUTH"):
			conn.Write([]byte("235 Authentication succeeded\r\n"))
		case strings.HasPrefix(line, "MAIL FROM:"):
			conn.Write([]byte("250 OK\r\n"))
		case strings.HasPrefix(line, "RCPT TO:"):
			conn.Write([]byte("250 OK\r\n"))
		case strings.HasPrefix(line, "DATA"):
			conn.Write([]byte("354 End data with <CR><LF>.<CR><LF>\r\n"))
		case line == ".":
			conn.Write([]byte("250 OK: queued as 12345\r\n"))
		case strings.HasPrefix(line, "QUIT"):
			conn.Write([]byte("221 Bye\r\n"))
			quit = true
		}
		if quit {
			break
		}
	}

	s.messages = append(s.messages, builder.String())
}

func (s *mockSMTPServer) stop() {
	if s.listener != nil {
		s.listener.Close()
	}
}

// -----------------------------------------------------------------------------
// Tests for SendEmailHandler
// -----------------------------------------------------------------------------

func TestSendEmailHandler_Success(t *testing.T) {
	// Setup mock SMTP server
	smtpServer := newMockSMTPServer()
	if err := smtpServer.start(); err != nil {
		t.Fatalf("Failed to start mock SMTP server: %v", err)
	}
	defer smtpServer.stop()

	// Set environment variables for email configuration
	os.Setenv("EMAIL_FROM", "test@example.com")
	os.Setenv("EMAIL_PASSWORD", "password")
	os.Setenv("EMAIL_HOST", "localhost")
	os.Setenv("EMAIL_PORT", "1025")

	// Create a temporary attachment file
	tmpFile, err := os.CreateTemp("", "attachment.txt")
	if err != nil {
		t.Fatalf("Failed to create temporary file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write some content to the temporary file
	_, err = tmpFile.WriteString("This is a test attachment.")
	if err != nil {
		t.Fatalf("Failed to write to temporary file: %v", err)
	}
	tmpFile.Close()

	// Create a sample email payload with attachment
	payload := models.SendEmailPayload{
		Recipient:   "recipient@example.com",
		Subject:     "Test Email",
		Body:        "This is a test email body.",
		Attachments: []string{tmpFile.Name()},
	}
	payloadBytes, _ := json.Marshal(payload)

	// Call the SendEmailHandler
	result, err := SendEmailHandler(payloadBytes)

	// Assertions on the result
	assert.NoError(t, err)
	assert.Equal(t, "Email sent successfully", result.Message)

	// Allow a brief moment for asynchronous processing
	time.Sleep(100 * time.Millisecond)

	// Verify that the mock SMTP server received the email
	if len(smtpServer.messages) == 0 {
		t.Fatal("Mock SMTP server did not receive any messages")
	}

	// Check that the email content includes key headers and parts.
	emailContent := smtpServer.messages[0]
	assert.Contains(t, emailContent, "To: recipient@example.com")
	assert.Contains(t, emailContent, "Subject: Test Email")
	assert.Contains(t, emailContent, "This is a test email body.")
	assert.Contains(t, emailContent, "Content-Disposition: attachment")
	assert.Contains(t, emailContent, "This is a test attachment.")
}

func TestSendEmailHandler_InvalidPayload(t *testing.T) {
	// Call the SendEmailHandler with an invalid payload
	_, err := SendEmailHandler([]byte("invalid payload"))

	// Assertions
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal send email payload")
}

func TestSendEmailHandler_MissingConfig(t *testing.T) {
	// Unset environment variables to simulate missing configuration
	os.Unsetenv("EMAIL_FROM")
	os.Unsetenv("EMAIL_PASSWORD")
	os.Unsetenv("EMAIL_HOST")
	os.Unsetenv("EMAIL_PORT")

	// Create a sample email payload
	payload := models.SendEmailPayload{
		Recipient: "recipient@example.com",
		Subject:   "Test Email",
		Body:      "This is a test email body.",
	}
	payloadBytes, _ := json.Marshal(payload)

	// Call the SendEmailHandler
	_, err := SendEmailHandler(payloadBytes)

	// Assertions
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "email configuration not complete")
}

func TestSendEmailHandler_AttachmentTooLarge(t *testing.T) {
	// Setup mock SMTP server
	smtpServer := newMockSMTPServer()
	if err := smtpServer.start(); err != nil {
		t.Fatalf("Failed to start mock SMTP server: %v", err)
	}
	defer smtpServer.stop()

	// Set environment variables for email configuration
	os.Setenv("EMAIL_FROM", "test@example.com")
	os.Setenv("EMAIL_PASSWORD", "password")
	os.Setenv("EMAIL_HOST", "localhost")
	os.Setenv("EMAIL_PORT", "2525")

	// Create a temporary attachment file that exceeds the size limit
	tmpFile, err := os.CreateTemp("", "large_attachment.txt")
	if err != nil {
		t.Fatalf("Failed to create temporary file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Create a large byte slice (maxAttachmentSize + 1)
	largeData := make([]byte, maxAttachmentSize+1)

	// Write the large data to the temporary file
	_, err = tmpFile.Write(largeData)
	if err != nil {
		t.Fatalf("Failed to write to temporary file: %v", err)
	}
	tmpFile.Close()

	// Create a sample email payload with the large attachment
	payload := models.SendEmailPayload{
		Recipient:   "recipient@example.com",
		Subject:     "Test Email",
		Body:        "This is a test email body.",
		Attachments: []string{tmpFile.Name()},
	}
	payloadBytes, _ := json.Marshal(payload)

	// Call the SendEmailHandler
	_, err = SendEmailHandler(payloadBytes)

	// Assertions
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "attachment size exceeds the limit")
}
