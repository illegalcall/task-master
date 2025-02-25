package jobs

type SendEmailPayload struct {
	Recipient    string            `json:"recipient"`
	Subject      string            `json:"subject"`
	Body         string            `json:"body"`
	TemplateName string            `json:"template_name,omitempty"`
	Attachments  map[string][]byte `json:"attachments,omitempty"` // filename: content
}
