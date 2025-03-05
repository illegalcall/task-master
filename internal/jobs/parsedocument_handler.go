package jobs

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

// Make these functions variables so they can be mocked in tests
var (
	ExtractPDFText = extractPDFTextImpl
	NewGeminiClient = newGeminiClientImpl
)

// GeminiClient is an interface for the Gemini LLM service
type GeminiClient interface {
	GenerateContent(ctx context.Context, text string, schema map[string]interface{}, description string) ([]byte, error)
}

// HTTPGeminiClient implements the GeminiClient interface using HTTP requests
type HTTPGeminiClient struct {
	apiKey string
	// Optional function for testing/mocking
	generateContentFunc func(ctx context.Context, text string, schema map[string]interface{}, description string) ([]byte, error)
}

// GeminiRequest represents a request to the Gemini API
type GeminiRequest struct {
	Contents []GeminiContent `json:"contents"`
}

// GeminiContent represents the content part of a Gemini request
type GeminiContent struct {
	Parts []GeminiPart `json:"parts"`
}

// GeminiPart represents a part of the content in a Gemini request
type GeminiPart struct {
	Text string `json:"text"`
}

// GeminiResponse represents a response from the Gemini API
type GeminiResponse struct {
	Candidates []GeminiCandidate `json:"candidates"`
}

// GeminiCandidate represents a candidate response from Gemini
type GeminiCandidate struct {
	Content struct {
		Parts []GeminiPart `json:"parts"`
	} `json:"content"`
}

// newGeminiClientImpl creates a new Gemini client using the API key from environment variables
func newGeminiClientImpl(ctx context.Context) (*HTTPGeminiClient, error) {
	apiKey := "AIzaSyD00N4RJfHBSqo1fLfzgKtGnl7NZ-Oy1Os"
	slog.Info("GEMINI_API_KEY", "apiKey", apiKey)
	if apiKey == "" {
		return nil, errors.New("GEMINI_API_KEY environment variable is not set")
	}

	return &HTTPGeminiClient{
		apiKey: apiKey,
	}, nil
}

// GenerateContent sends a request to Gemini to convert extracted text into structured JSON
func (c *HTTPGeminiClient) GenerateContent(ctx context.Context, text string, schema map[string]interface{}, description string) ([]byte, error) {
	// If there's a test override function, use it instead
	if c.generateContentFunc != nil {
		return c.generateContentFunc(ctx, text, schema, description)
	}
	
	// Convert schema to a readable string format
	schemaBytes, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal schema: %w", err)
	}
	schemaStr := string(schemaBytes)

	// Build the prompt for the model
	prompt := fmt.Sprintf(`
Extract structured data from the following document text according to the provided JSON schema.
Use the description to guide your extraction.

DESCRIPTION:
%s

JSON SCHEMA:
%s

DOCUMENT TEXT:
%s

Respond with ONLY a valid JSON object matching the schema. Do not include any explanations or markdown formatting.
`, description, schemaStr, text)

	// Create the request to Gemini API
	reqBody := GeminiRequest{
		Contents: []GeminiContent{
			{
				Parts: []GeminiPart{
					{
						Text: prompt,
					},
				},
			},
		},
	}

	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Gemini API endpoint
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-pro:generateContent?key=%s", c.apiKey)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse the response
	var geminiResp GeminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return nil, errors.New("no response generated")
	}

	// Extract the response text
	responseText := geminiResp.Candidates[0].Content.Parts[0].Text
	
	// Clean up response - remove any markdown code block formatting
	cleanResponse := strings.TrimSpace(responseText)
	cleanResponse = strings.TrimPrefix(cleanResponse, "```json")
	cleanResponse = strings.TrimPrefix(cleanResponse, "```")
	cleanResponse = strings.TrimSuffix(cleanResponse, "```")
	cleanResponse = strings.TrimSpace(cleanResponse)

	// Validate the response is valid JSON
	var jsonResponse interface{}
	if err := json.Unmarshal([]byte(cleanResponse), &jsonResponse); err != nil {
		return nil, fmt.Errorf("invalid JSON response from LLM: %w", err)
	}

	return []byte(cleanResponse), nil
}

// SimplePDFExtractor extracts text from a PDF file
// Note: In a real implementation, you would use a proper PDF parsing library
func SimplePDFExtractor(filePath string) (string, error) {
	// In a real implementation, you would use a PDF library here
	// For now, we'll just return a placeholder or read the file as text
	// to avoid external dependencies
	
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	
	// For simplicity, we'll just return the first part of the file as text
	// In a real implementation, you would parse the PDF properly
	return fmt.Sprintf("PDF Content (simulated): %s", string(content[:min(len(content), 1000)])), nil
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// extractPDFTextImpl extracts text content from a PDF document
func extractPDFTextImpl(documentSource string, documentType string, maxPages int) (string, error) {
	slog.Info("Starting PDF text extraction", "documentType", documentType, "documentSource", documentSource)
	switch documentType {
	case "path":
		slog.Info("Using simple extractor for local file path", "documentSource", documentSource)
		text, err := SimplePDFExtractor(documentSource)
		if err != nil {
			slog.Info("SimplePDFExtractor failed for local file", "documentSource", documentSource, "error", err)
		} else {
			slog.Info("SimplePDFExtractor succeeded for local file", "documentSource", documentSource)
		}
		return text, err

	case "url":
		slog.Info("Downloading PDF from URL", "documentSource", documentSource)
		resp, err := http.Get(documentSource)
		if err != nil {
			slog.Info("Failed to download file", "documentSource", documentSource, "error", err)
			return "", fmt.Errorf("failed to download file: %w", err)
		}
		defer resp.Body.Close()
		slog.Info("File downloaded successfully", "documentSource", documentSource)

		tempFile, err := ioutil.TempFile("", "pdf-*.pdf")
		if err != nil {
			slog.Info("Failed to create temporary file for URL download", "error", err)
			return "", fmt.Errorf("failed to create temp file: %w", err)
		}
		slog.Info("Temporary file created", "tempFile", tempFile.Name())
		defer os.Remove(tempFile.Name())
		defer tempFile.Close()

		_, err = io.Copy(tempFile, resp.Body)
		if err != nil {
			slog.Info("Failed to write downloaded content to temporary file", "tempFile", tempFile.Name(), "error", err)
			return "", fmt.Errorf("failed to write downloaded content: %w", err)
		}
		slog.Info("Downloaded content written to temporary file", "tempFile", tempFile.Name())

		tempFile.Close() // Close to flush writes
		slog.Info("Temporary file closed", "tempFile", tempFile.Name())

		text, err := SimplePDFExtractor(tempFile.Name())
		if err != nil {
			slog.Info("SimplePDFExtractor failed for file downloaded from URL", "tempFile", tempFile.Name(), "error", err)
		} else {
			slog.Info("SimplePDFExtractor succeeded for file downloaded from URL", "tempFile", tempFile.Name())
		}
		return text, err

	case "base64":
		slog.Info("Decoding base64 PDF content")
		decoded, err := base64.StdEncoding.DecodeString(documentSource)
		if err != nil {
			slog.Info("Failed to decode base64 content", "error", err)
			return "", fmt.Errorf("failed to decode base64: %w", err)
		}
		slog.Info("Base64 content decoded successfully")

		tempFile, err := ioutil.TempFile("", "pdf-*.pdf")
		if err != nil {
			slog.Info("Failed to create temporary file for base64 content", "error", err)
			return "", fmt.Errorf("failed to create temp file: %w", err)
		}
		slog.Info("Temporary file created for base64 content", "tempFile", tempFile.Name())
		defer os.Remove(tempFile.Name())
		defer tempFile.Close()

		if _, err := tempFile.Write(decoded); err != nil {
			slog.Info("Failed to write decoded base64 content to temporary file", "tempFile", tempFile.Name(), "error", err)
			return "", fmt.Errorf("failed to write to temp file: %w", err)
		}
		slog.Info("Decoded base64 content written to temporary file", "tempFile", tempFile.Name())
		tempFile.Close() // Close to flush writes
		slog.Info("Temporary file closed", "tempFile", tempFile.Name())

		text, err := SimplePDFExtractor(tempFile.Name())
		if err != nil {
			slog.Info("SimplePDFExtractor failed for file created from base64", "tempFile", tempFile.Name(), "error", err)
		} else {
			slog.Info("SimplePDFExtractor succeeded for file created from base64", "tempFile", tempFile.Name())
		}
		return text, err

	default:
		slog.Info("Unsupported document type encountered", "documentType", documentType)
		return "", fmt.Errorf("unsupported document type: %s", documentType)
	}
}


// Global tracker instance
var globalTracker *ParsingTracker

// InitParsingTracker initializes the global parsing tracker
func InitParsingTracker(config ParsingTrackerConfig) {
	globalTracker = NewParsingTracker(config)
}

// GetParsingTracker returns the global parsing tracker
func GetParsingTracker() *ParsingTracker {
	if globalTracker == nil {
		globalTracker = NewParsingTracker(DefaultParsingTrackerConfig())
	}
	return globalTracker
}

// ParseDocumentWithTracking handles document parsing jobs with status tracking and retries
func ParseDocumentWithTracking(ctx context.Context, payload []byte) (Result, error) {
	slog.Info("ParseDocumentWithTracking started", "payload", string(payload))
	// Parse the payload to get the document ID
	var parsedPayload struct {
		DocumentID string `json:"documentID"`
		DocumentType string `json:"documentType"`
		DocumentSource string `json:"documentSource"`
		ExpectedSchema string `json:"expected_schema"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(payload, &parsedPayload); err != nil {
		slog.Info("Failed to extract documentID from payload", "error", err)
		return Result{}, fmt.Errorf("failed to extract document ID: %w", err)
	}

	documentID := parsedPayload.DocumentID
	documentType := parsedPayload.DocumentType
	documentSource := parsedPayload.DocumentSource
	expectedSchema := parsedPayload.ExpectedSchema
	description := parsedPayload.Description
	if documentID == "" {
		slog.Info("documentID is missing in payload")
		return Result{}, fmt.Errorf("documentID is required")
	}
	slog.Info("DocumentID extracted", "documentID", documentID)

	tracker := GetParsingTracker()
	slog.Info("Parsing tracker obtained", "documentID", documentID)
	
	// Update status to uploaded if this is the first time
	tracker.UpdateStatus(documentID, StatusUploaded, nil)
	slog.Info("Tracker status updated to 'uploaded'", "documentID", documentID)

	// Handle any panics by updating status before re-panicking
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("panic recovered: %v", r)
			tracker.UpdateStatus(documentID, StatusFailed, err)
			slog.Info("Panic recovered, tracker status updated to 'failed'", "documentID", documentID, "error", err)
			panic(r)
		}
	}()

	var result Result
	var finalErr error

	// Retry loop
	maxAttempts := tracker.config.MaxRetries + 1 // +1 for the initial attempt
	slog.Info("Starting retry loop", "maxAttempts", maxAttempts, "documentID", documentID)
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		start := time.Now()
		slog.Info("Retry attempt started", "attempt", attempt, "documentID", documentID)
		
		// If this is a retry attempt, update status to retrying and delay briefly
		if attempt > 1 {
			tracker.UpdateStatus(documentID, StatusRetrying, nil)
			slog.Info("Tracker status updated to 'retrying'", "documentID", documentID, "attempt", attempt)
			time.Sleep(time.Millisecond * 100)
		}
		
		// Update status to parsing
		tracker.UpdateStatus(documentID, StatusParsing, nil)
		slog.Info("Tracker status updated to 'parsing'", "documentID", documentID, "attempt", attempt)
		
		// Extract and validate the document
		var parsedPayload ParseDocumentPayload
		if err := json.Unmarshal(payload, &parsedPayload); err != nil {
			finalErr = fmt.Errorf("failed to unmarshal payload: %w", err)
			tracker.UpdateStatus(documentID, StatusFailed, finalErr)
			slog.Info("Failed to unmarshal payload during parsing", "documentID", documentID, "attempt", attempt, "error", finalErr)
			continue // Try again if retries are available
		}
		slog.Info("Payload unmarshalled successfully for parsing", "parsedPayload", parsedPayload)

		// Extract text from the PDF
		tracker.UpdateStatus(documentID, StatusParsing, nil)
		slog.Info("Tracker status updated to 'parsing' for text extraction", "documentID", documentID, "attempt", attempt)
		maxPages := parsedPayload.Options.MaxPages
		text, err := ExtractPDFText(documentSource, documentType, maxPages)
		if err != nil {
			finalErr = fmt.Errorf("text extraction error: %w", err)
			tracker.UpdateStatus(documentID, StatusFailed, finalErr)
			slog.Info("Text extraction failed", "documentID", documentID, "attempt", attempt, "error", finalErr)
			continue // Try again if retries are available
		}
		slog.Info("Text extraction succeeded", "documentID", documentID, "attempt", attempt, "extractedTextLen", len(text))

		// Process with LLM
		tracker.UpdateStatus(documentID, StatusConverting, nil)
		slog.Info("Tracker status updated to 'converting'", "documentID", documentID, "attempt", attempt)
		geminiClient, err := NewGeminiClient(ctx)
		if err != nil {
			finalErr = fmt.Errorf("failed to initialize Gemini client: %w", err)
			tracker.UpdateStatus(documentID, StatusFailed, finalErr)
			slog.Info("Failed to initialize Gemini client", "documentID", documentID, "attempt", attempt, "error", finalErr)
			continue // Try again if retries are available
		}
		slog.Info("Gemini client initialized", "documentID", documentID, "attempt", attempt)
		//log the text,outputSchema,description
		slog.Info("Text", "text", text)
		slog.Info("OutputSchema", "outputSchema", expectedSchema)
		slog.Info("Description", "description", description)
		structuredData, err := geminiClient.GenerateContent(
			ctx,
			text,
			parsedPayload.OutputSchema,
			parsedPayload.Description,
		)
		if err != nil {
			finalErr = fmt.Errorf("LLM processing error: %w", err)
			tracker.UpdateStatus(documentID, StatusFailed, finalErr)
			slog.Info("LLM processing failed", "documentID", documentID, "attempt", attempt, "error", finalErr)
			continue // Try again if retries are available
		}
		slog.Info("LLM processing succeeded", "documentID", documentID, "attempt", attempt)

		// Parse the structured data into our response format
		var parsedContent interface{}
		if err := json.Unmarshal(structuredData, &parsedContent); err != nil {
			finalErr = fmt.Errorf("failed to parse LLM response: %w", err)
			tracker.UpdateStatus(documentID, StatusFailed, finalErr)
			slog.Info("Failed to parse LLM response", "documentID", documentID, "attempt", attempt, "error", finalErr)
			continue // Try again if retries are available
		}
		slog.Info("LLM response parsed successfully", "documentID", documentID, "attempt", attempt)

		// Calculate processing time
		elapsedTime := time.Since(start)
		slog.Info("Processing time calculated", "documentID", documentID, "attempt", attempt, "elapsedTimeMs", elapsedTime.Milliseconds())
		
		// Update metrics and construct the parsed document
		parsedDocument := ParsedDocument{
			Content: parsedContent,
			MetaInfo: map[string]interface{}{
				"processingTimeMs": elapsedTime.Milliseconds(),
				"documentType":     parsedPayload.DocumentType,
				"extractedTextLen": len(text),
				"attempts":         attempt,
			},
		}
		slog.Info("Parsed document metrics collected", "documentID", documentID, "attempt", attempt, "metaInfo", parsedDocument.MetaInfo)

		result = Result{
			Data: parsedDocument,
			Metadata: map[string]interface{}{
				"completedAt": time.Now().Format(time.RFC3339),
				"success":     true,
				"documentID":  documentID,
			},
		}
		slog.Info("Result constructed successfully", "documentID", documentID, "attempt", attempt)

		// Update status to complete
		tracker.UpdateStatus(documentID, StatusComplete, nil)
		slog.Info("Tracker status updated to 'complete'", "documentID", documentID, "attempt", attempt)
		
		// Success, exit the retry loop
		finalErr = nil
		break
	}

	if finalErr != nil {
		slog.Info("Final error after retries", "documentID", documentID, "error", finalErr)
		return Result{}, finalErr
	}

	slog.Info("ParseDocumentWithTracking completed successfully", "documentID", documentID)
	return result, nil
}


// ParseDocumentHandler handles document parsing jobs
func ParseDocumentHandler(ctx context.Context, payload []byte) (Result, error) {
	slog.Info("ParseDocumentHandler invoked", "payload", string(payload))
	
	// Attempt to extract document ID from payload
	var docIDContainer struct {
		DocumentID string `json:"documentID"`
	}
	if err := json.Unmarshal(payload, &docIDContainer); err != nil {
		slog.Error("Failed to unmarshal payload for documentID extraction", "error", err)
		slog.Info("Falling back to simpleParseDocument due to unmarshalling error")
		return simpleParseDocument(ctx, payload)
	}
	slog.Info("Extracted documentID container", "documentID", docIDContainer.DocumentID)
	
	documentID := docIDContainer.DocumentID
	if documentID == "" {
		slog.Info("No documentID found in payload, generating a new one")
		documentID = fmt.Sprintf("doc-%s", time.Now().Format("20060102-150405-999999"))
		slog.Info("Generated documentID", "documentID", documentID)
		
		// Add the generated documentID to the payload
		var parsedPayload map[string]interface{}
		if err := json.Unmarshal(payload, &parsedPayload); err != nil {
			slog.Error("Failed to unmarshal payload into map for documentID insertion", "error", err)
			slog.Info("Falling back to simpleParseDocument due to unmarshalling error on map")
			return simpleParseDocument(ctx, payload)
		}
		parsedPayload["documentID"] = documentID
		parsedPayload["documentType"] = "url"
		parsedPayload["documentSource"]=parsedPayload["pdf_source"]
		slog.Info("Inserted documentID into payload map", "documentID", documentID, "payloadMap", parsedPayload,"documentType", parsedPayload["documentType"],"documentSource", parsedPayload["documentSource"])
		
		// Reconstruct the payload with the documentID included
		updatedPayload, err := json.Marshal(parsedPayload)
		if err != nil {
			slog.Error("Failed to marshal updated payload with documentID", "error", err)
			slog.Info("Falling back to simpleParseDocument due to marshalling error")
			return simpleParseDocument(ctx, payload)
		}
		slog.Info("Reconstructed payload with documentID", "updatedPayload", string(updatedPayload))
		payload = updatedPayload
	} else {
		slog.Info("DocumentID found in payload", "documentID", documentID)
	}

	// Proceed with tracking system for parsing
	slog.Info("Invoking ParseDocumentWithTracking", "documentID", documentID)
	result, err := ParseDocumentWithTracking(ctx, payload)
	if err != nil {
		slog.Error("ParseDocumentWithTracking failed", "documentID", documentID, "error", err)
	} else {
		slog.Info("ParseDocumentWithTracking succeeded", "documentID", documentID)
	}
	return result, err
}

// simpleParseDocument implements the original document parsing logic without tracking
func simpleParseDocument(ctx context.Context, payload []byte) (Result, error) {
	start := time.Now()

	// 1. Parse and validate the payload
	if err := ValidateWithGJSON(payload); err != nil {
		return Result{}, fmt.Errorf("invalid payload: %w", err)
	}

	var parsedPayload ParseDocumentPayload
	if err := json.Unmarshal(payload, &parsedPayload); err != nil {
		return Result{}, fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	// 2. Extract text from the PDF
	maxPages := parsedPayload.Options.MaxPages
	text, err := ExtractPDFText(parsedPayload.Document, parsedPayload.DocumentType, maxPages)
	if err != nil {
		return Result{}, fmt.Errorf("text extraction error: %w", err)
	}

	// 3. Process the extracted text with LLM (Gemini)
	geminiClient, err := NewGeminiClient(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("failed to initialize Gemini client: %w", err)
	}

	structuredData, err := geminiClient.GenerateContent(
		ctx,
		text,
		parsedPayload.OutputSchema,
		parsedPayload.Description,
	)
	if err != nil {
		return Result{}, fmt.Errorf("LLM processing error: %w", err)
	}

	// 4. Parse the structured data into our response format
	var parsedContent interface{}
	if err := json.Unmarshal(structuredData, &parsedContent); err != nil {
		return Result{}, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	// 5. Prepare the result
	elapsedTime := time.Since(start)
	parsedDocument := ParsedDocument{
		Content: parsedContent,
		MetaInfo: map[string]interface{}{
			"processingTimeMs": elapsedTime.Milliseconds(),
			"documentType":     parsedPayload.DocumentType,
			"extractedTextLen": len(text),
		},
	}

	result := Result{
		Data: parsedDocument,
		Metadata: map[string]interface{}{
			"completedAt": time.Now().Format(time.RFC3339),
			"success":     true,
		},
	}

	return result, nil
} 