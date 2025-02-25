package models

type SendEmailPayload struct {
	Recipient string `json:"recipient"`
	Subject string `json:"subject"`
	Body string `json:"body"`
	TemplateName string `json:"template_name"`
	Attachments []string `json:"attachments"` // Assuming file paths for simplicity
}