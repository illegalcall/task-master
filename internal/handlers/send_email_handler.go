package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/smtp"
	"net/textproto"
	"os"
	"path/filepath"

	"github.com/illegalcall/task-master/internal/models"
)

type EmailConfig struct {
	From     string
	Password string
	Host     string
	Port     int
}

const (
	maxAttachmentSize = 10 * 1024 * 1024 // 10MB
)

func LoadEmailConfig() (*EmailConfig, error) {
	from := os.Getenv("EMAIL_FROM")
	password := os.Getenv("EMAIL_PASSWORD")
	host := os.Getenv("EMAIL_HOST")
	portStr := os.Getenv("EMAIL_PORT")

	log.Printf("Testing logger for: %s", from)

	if from == "" || password == "" || host == "" || portStr == "" {
		return nil, fmt.Errorf("email configuration not complete")
	}

	var port int
	_, err := fmt.Sscan(portStr, &port)
	if err != nil {
		return nil, fmt.Errorf("failed to parse EMAIL_PORT: %w", err)
	}

	return &EmailConfig{
		From:     from,
		Password: password,
		Host:     host,
		Port:     port,
	}, nil
}

func SendEmailHandler(payload []byte) (models.Result, error) {
	// Load email configuration
	emailConfig, err := LoadEmailConfig()
	if err != nil {
		return models.Result{}, fmt.Errorf("failed to load email config: %w", err)
	}

	// Unmarshal the payload
	var emailPayload models.SendEmailPayload
	if err := json.Unmarshal(payload, &emailPayload); err != nil {
		return models.Result{}, fmt.Errorf("failed to unmarshal send email payload: %w", err)
	}

	// Validate payload
	if emailPayload.Recipient == "" || emailPayload.Subject == "" {
		return models.Result{}, fmt.Errorf("recipient and subject are required")
	}

	// Prepare the email body
	body := ""
	if emailPayload.TemplateName != "" {
		tmpl, err := template.ParseFiles(filepath.Join("templates", emailPayload.TemplateName+".html"))
		if err != nil {
			return models.Result{}, fmt.Errorf("failed to parse template file: %w", err)
		}

		var tpl bytes.Buffer
		if err := tmpl.Execute(&tpl, emailPayload); err != nil {
			return models.Result{}, fmt.Errorf("failed to execute template: %w", err)
		}
		body = tpl.String()
	} else {
		body = emailPayload.Body
	}

	// Create an authentication object.
	auth := smtp.PlainAuth("", emailConfig.From, emailConfig.Password, emailConfig.Host)

	// Create a new multipart writer
	var msg bytes.Buffer
	mw := multipart.NewWriter(&msg)

	// Set email headers
	msg.WriteString("MIME-version: 1.0;\r\n")
	msg.WriteString(fmt.Sprintf("From: %s\r\n", emailConfig.From))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", emailPayload.Recipient))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", emailPayload.Subject))
	msg.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=%s\r\n", mw.Boundary()))
	msg.WriteString("\r\n")

	// Add the email body
	h := make(textproto.MIMEHeader)
	h.Set("Content-Type", "text/html")
	pw, err := mw.CreatePart(h)
	if err != nil {
		return models.Result{}, fmt.Errorf("failed to create email body part: %w", err)
	}
	if _, err = pw.Write([]byte(body)); err != nil {
		return models.Result{}, fmt.Errorf("failed to write email body: %w", err)
	}

	// Handle attachments
	for _, attachmentPath := range emailPayload.Attachments {
		// Open attachment file
		attachment, err := os.Open(attachmentPath)
		if err != nil {
			return models.Result{}, fmt.Errorf("failed to open attachment: %w", err)
		}
		defer attachment.Close()

		// Get attachment file info
		fileInfo, err := attachment.Stat()
		if err != nil {
			return models.Result{}, fmt.Errorf("failed to get attachment info: %w", err)
		}

		// Check attachment size
		if fileInfo.Size() > maxAttachmentSize {
			return models.Result{}, fmt.Errorf("attachment size exceeds the limit of %dMB", maxAttachmentSize/1024/1024)
		}

		// Create attachment header
		h := make(textproto.MIMEHeader)
		h.Set("Content-Type", mime.TypeByExtension(filepath.Ext(attachmentPath)))
		h.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filepath.Base(attachmentPath)))

		// Create attachment part
		ap, err := mw.CreatePart(h)
		if err != nil {
			return models.Result{}, fmt.Errorf("failed to create attachment part: %w", err)
		}

		// Copy attachment data
		if _, err = io.Copy(ap, attachment); err != nil {
			return models.Result{}, fmt.Errorf("failed to copy attachment data: %w", err)
		}
	}

	// Close the multipart writer
	if err := mw.Close(); err != nil {
		return models.Result{}, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// Send the email
	err = smtp.SendMail(
		fmt.Sprintf("%s:%d", emailConfig.Host, emailConfig.Port),
		auth,
		emailConfig.From,
		[]string{emailPayload.Recipient},
		msg.Bytes(),
	)

	if err != nil {
		return models.Result{}, fmt.Errorf("failed to send email: %w", err)
	}

	slog.Info("Email sent successfully", "recipient", emailPayload.Recipient, "subject", emailPayload.Subject)

	return models.Result{
		Message: "Email sent successfully",
		Data:    map[string]interface{}{"recipient": emailPayload.Recipient, "subject": emailPayload.Subject},
	}, nil
}
