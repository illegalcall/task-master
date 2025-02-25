package models

import (
	"encoding/json"
	"testing"
)

func TestSendEmailPayload_Serialization(t *testing.T) {
	payload := SendEmailPayload{
		Recipient:    "test@example.com",
		Subject:      "Test Email",
		Body:         "This is a test email body.",
		TemplateName: "welcome",
		Attachments:  []string{"/path/to/attachment1.txt", "/path/to/attachment2.pdf"},
	}

	// Serialize to JSON
	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Failed to serialize payload: %v", err)
	}

	// Deserialize from JSON
	var deserializedPayload SendEmailPayload
	err = json.Unmarshal(jsonBytes, &deserializedPayload)
	if err != nil {
		t.Fatalf("Failed to deserialize payload: %v", err)
	}

	// Verify the fields
	if payload.Recipient != deserializedPayload.Recipient {
		t.Errorf("Recipient mismatch: expected %s, got %s", payload.Recipient, deserializedPayload.Recipient)
	}
	if payload.Subject != deserializedPayload.Subject {
		t.Errorf("Subject mismatch: expected %s, got %s", payload.Subject, deserializedPayload.Subject)
	}
	if payload.Body != deserializedPayload.Body {
		t.Errorf("Body mismatch: expected %s, got %s", payload.Body, deserializedPayload.Body)
	}
	if payload.TemplateName != deserializedPayload.TemplateName {
		t.Errorf("TemplateName mismatch: expected %s, got %s", payload.TemplateName, deserializedPayload.TemplateName)
	}

	// Verify attachments
	if len(payload.Attachments) != len(deserializedPayload.Attachments) {
		t.Errorf("Attachments length mismatch: expected %d, got %d", len(payload.Attachments), len(deserializedPayload.Attachments))
	}
	for i, attachment := range payload.Attachments {
		if attachment != deserializedPayload.Attachments[i] {
			t.Errorf("Attachment mismatch at index %d: expected %s, got %s", i, attachment, deserializedPayload.Attachments[i])
		}
	}
}
