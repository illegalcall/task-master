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
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/illegalcall/task-master/internal/models"
)

// Make these functions variables so they can be mocked in tests
var (
	ExtractPDFText  = extractPDFTextImpl
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
	apiKey := os.Getenv("GEMINI_API_KEY")
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
	fmt.Println("documentSource", documentSource)
	fmt.Println("documentType", documentType)
	fmt.Println("maxPages", maxPages)

	switch documentType {
	case "path":
		// For simplicity, we'll just use our simple extractor
		return SimplePDFExtractor(documentSource)

	case "url":
		// Download the file to a temporary location
		resp, err := http.Get(documentSource)
		if err != nil {
			return "", fmt.Errorf("failed to download file: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("failed to download file: HTTP status %d", resp.StatusCode)
		}

		tempFile, err := ioutil.TempFile("", "pdf-*.pdf")
		if err != nil {
			return "", fmt.Errorf("failed to create temp file: %w", err)
		}
		defer os.Remove(tempFile.Name())
		defer tempFile.Close()

		_, err = io.Copy(tempFile, resp.Body)
		if err != nil {
			return "", fmt.Errorf("failed to write downloaded content: %w", err)
		}
		tempFile.Close() // Close to flush writes

		return SimplePDFExtractor(tempFile.Name())

	case "base64":
		// Decode base64 content
		decoded, err := base64.StdEncoding.DecodeString(documentSource)
		if err != nil {
			return "", fmt.Errorf("failed to decode base64: %w", err)
		}

		// Save to temporary file
		tempFile, err := ioutil.TempFile("", "pdf-*.pdf")
		if err != nil {
			return "", fmt.Errorf("failed to create temp file: %w", err)
		}
		defer os.Remove(tempFile.Name())
		defer tempFile.Close()

		if _, err := tempFile.Write(decoded); err != nil {
			return "", fmt.Errorf("failed to write to temp file: %w", err)
		}
		tempFile.Close() // Close to flush writes

		return SimplePDFExtractor(tempFile.Name())

	default:
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
	// Parse the payload to get the document ID
	var parsedPayload struct {
		DocumentID string `json:"documentID"`
	}
	if err := json.Unmarshal(payload, &parsedPayload); err != nil {
		return Result{}, fmt.Errorf("failed to extract document ID: %w", err)
	}

	documentID := parsedPayload.DocumentID
	if documentID == "" {
		return Result{}, errors.New("documentID is required")
	}

	tracker := GetParsingTracker()

	// Update status to uploaded if this is the first time
	tracker.UpdateStatus(documentID, StatusUploaded, nil)

	// Function to handle any panics by updating status
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("panic recovered: %v", r)
			tracker.UpdateStatus(documentID, StatusFailed, err)
			// Re-panic so it can be handled by higher-level recovery mechanisms
			panic(r)
		}
	}()

	var result Result
	var finalErr error

	// Retry loop
	maxAttempts := tracker.config.MaxRetries + 1 // +1 for the initial attempt
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		start := time.Now()

		// If this is a retry attempt, update status to retrying
		if attempt > 1 {
			tracker.UpdateStatus(documentID, StatusRetrying, nil)
			// Add a small delay before retrying to prevent hammering the system
			time.Sleep(time.Millisecond * 100)
		}

		// Update status to parsing
		tracker.UpdateStatus(documentID, StatusParsing, nil)

		// Extract and validate the document
		var parsedPayload ParseDocumentPayload
		if err := json.Unmarshal(payload, &parsedPayload); err != nil {
			finalErr = fmt.Errorf("failed to unmarshal payload: %w", err)
			tracker.UpdateStatus(documentID, StatusFailed, finalErr)
			continue // Try again if retries are available
		}

		// Extract text from the PDF
		tracker.UpdateStatus(documentID, StatusParsing, nil)
		maxPages := parsedPayload.Options.MaxPages
		text, err := ExtractPDFText(parsedPayload.Document, parsedPayload.DocumentType, maxPages)
		if err != nil {
			finalErr = fmt.Errorf("text extraction error: %w", err)
			tracker.UpdateStatus(documentID, StatusFailed, finalErr)
			continue // Try again if retries are available
		}

		// Process with LLM
		tracker.UpdateStatus(documentID, StatusConverting, nil)
		geminiClient, err := NewGeminiClient(ctx)
		if err != nil {
			finalErr = fmt.Errorf("failed to initialize Gemini client: %w", err)
			tracker.UpdateStatus(documentID, StatusFailed, finalErr)
			continue // Try again if retries are available
		}

		structuredData, err := geminiClient.GenerateContent(
			ctx,
			text,
			parsedPayload.OutputSchema,
			parsedPayload.Description,
		)
		if err != nil {
			finalErr = fmt.Errorf("LLM processing error: %w", err)
			tracker.UpdateStatus(documentID, StatusFailed, finalErr)
			continue // Try again if retries are available
		}

		// Parse the structured data into our response format
		var parsedContent interface{}
		if err := json.Unmarshal(structuredData, &parsedContent); err != nil {
			finalErr = fmt.Errorf("failed to parse LLM response: %w", err)
			tracker.UpdateStatus(documentID, StatusFailed, finalErr)
			continue // Try again if retries are available
		}

		// Calculate processing time
		elapsedTime := time.Since(start)

		// Update metrics
		parsedDocument := ParsedDocument{
			Content: parsedContent,
			MetaInfo: map[string]interface{}{
				"processingTimeMs": elapsedTime.Milliseconds(),
				"documentType":     parsedPayload.DocumentType,
				"extractedTextLen": len(text),
				"attempts":         attempt,
			},
		}

		result = Result{
			Data: parsedDocument,
			Metadata: map[string]interface{}{
				"completedAt": time.Now().Format(time.RFC3339),
				"success":     true,
				"documentID":  documentID,
			},
		}

		// Update status to complete
		tracker.UpdateStatus(documentID, StatusComplete, nil)

		// Success, exit the retry loop
		finalErr = nil
		break
	}

	if finalErr != nil {
		return Result{}, finalErr
	}

	return result, nil
}

// transformPayload converts from models.ParseDocumentPayload to jobs.ParseDocumentPayload
func transformPayload(payload []byte) ([]byte, error) {
	var modelPayload struct {
		models.ParseDocumentPayload
		PDFPath string `json:"pdf_path"`
	}

	if err := json.Unmarshal(payload, &modelPayload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal model payload: %w", err)
	}

	// Determine document type based on PDF source
	var documentType string
	var documentSource string
	if strings.HasPrefix(modelPayload.PDFSource, "http://") || strings.HasPrefix(modelPayload.PDFSource, "https://") {
		documentType = "url"
		documentSource = modelPayload.PDFSource
	} else if modelPayload.PDFPath != "" {
		documentType = "path"
		documentSource = modelPayload.PDFPath
	} else {
		documentType = "base64"
		documentSource = modelPayload.PDFSource
	}

	// Create the jobs payload
	jobPayload := ParseDocumentPayload{
		Document:     documentSource,
		DocumentType: documentType,
		OutputSchema: make(map[string]interface{}),
		Description:  modelPayload.Description,
		Options: ParseOptions{
			Language: modelPayload.Options.Language,
		},
	}

	// Convert ExpectedSchema to OutputSchema
	if err := json.Unmarshal(modelPayload.ExpectedSchema, &jobPayload.OutputSchema); err != nil {
		return nil, fmt.Errorf("failed to unmarshal expected schema: %w", err)
	}

	// Marshal the transformed payload
	transformedPayload, err := json.Marshal(jobPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal transformed payload: %w", err)
	}

	return transformedPayload, nil
}

// ParseDocumentHandler handles document parsing jobs
func ParseDocumentHandler(ctx context.Context, payload []byte) (Result, error) {
	fmt.Println("Received payload:", string(payload))

	// Transform the payload from models format to jobs format
	transformedPayload, err := transformPayload(payload)
	if err != nil {
		fmt.Printf("Failed to transform payload: %v\n", err)
		return Result{}, fmt.Errorf("failed to transform payload: %w", err)
	}

	fmt.Println("Transformed payload:", string(transformedPayload))

	// Extract document ID or generate one if not present
	var docIDContainer struct {
		DocumentID string `json:"documentID"`
	}

	if err := json.Unmarshal(transformedPayload, &docIDContainer); err != nil {
		fmt.Printf("Failed to unmarshal document ID: %v\n", err)
		// If we can't extract a document ID, we'll just use the regular parsing flow
		// without tracking
		return simpleParseDocument(ctx, transformedPayload)
	}

	documentID := docIDContainer.DocumentID
	if documentID == "" {
		// If no document ID is provided, generate a random one for tracking
		documentID = fmt.Sprintf("doc-%s", time.Now().Format("20060102-150405-999999"))
		fmt.Printf("Generated document ID: %s\n", documentID)

		// Add the document ID to the payload
		var parsedPayload map[string]interface{}
		if err := json.Unmarshal(transformedPayload, &parsedPayload); err != nil {
			fmt.Printf("Failed to unmarshal payload for document ID: %v\n", err)
			return simpleParseDocument(ctx, transformedPayload)
		}
		parsedPayload["documentID"] = documentID

		// Reconstruct the payload with the document ID
		updatedPayload, err := json.Marshal(parsedPayload)
		if err != nil {
			fmt.Printf("Failed to marshal updated payload: %v\n", err)
			return simpleParseDocument(ctx, transformedPayload)
		}

		transformedPayload = updatedPayload
		fmt.Println("Updated payload with document ID:", string(transformedPayload))
	}

	// Use the tracking system for parsing
	return ParseDocumentWithTracking(ctx, transformedPayload)
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
