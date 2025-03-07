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
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)


var llamaCloudAPIKey = os.Getenv("LLAMA_API_KEY")

// Make these functions variables so they can be mocked in tests
var (
	ExtractPDFText = extractPDFTextImpl
	NewGeminiClient = newGeminiClientImpl
)

// GeminiClient is an interface for the Gemini LLM service
type GeminiClient interface {
	GenerateContent(ctx context.Context, text string, schema string, description string) ([]byte, error)
}

// HTTPGeminiClient implements the GeminiClient interface using the official genai package
type HTTPGeminiClient struct {
	client *genai.Client
	model *genai.GenerativeModel
	// Optional function for testing/mocking
	generateContentFunc func(ctx context.Context, text string, schema string, description string) ([]byte, error)
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
	slog.Info("GEMINI_API_KEY", "apiKey", apiKey)
	if apiKey == "" {
		return nil, errors.New("GEMINI_API_KEY environment variable is not set")
	}

	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create genai client: %w", err)
	}

	model := client.GenerativeModel("gemini-2.0-flash")

	return &HTTPGeminiClient{
		client: client,
		model: model,
	}, nil
}

// GenerateContent sends a request to Gemini to convert extracted text into structured JSON
func (c *HTTPGeminiClient) GenerateContent(ctx context.Context, text string, schema string, description string) ([]byte, error) {
	// If there's a test override function, use it instead
	slog.Info("Generating content with genai package", "text length", len(text), "schema", schema, "description", description)

	if c.generateContentFunc != nil {
		return c.generateContentFunc(ctx, text, schema, description)
	}
	
	// Convert schema to a readable string format
	schemaBytes, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		slog.Info("Failed to marshal schema to string", "error", err)
		return nil, fmt.Errorf("failed to marshal schema: %w", err)
	}
	schemaStr := string(schemaBytes)
	slog.Info("Schema formatted for prompt", "schemaLength", len(schemaStr))

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
	slog.Info("Prompt built for Gemini", "promptLength", len(prompt))

	// Use the genai client to generate content
	resp, err := c.model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		slog.Info("Gemini API request failed", "error", err, "modelName")
		return nil, fmt.Errorf("failed to generate content: %w", err)
	}
	slog.Info("Received response from Gemini API", "candidatesCount", len(resp.Candidates))

	if len(resp.Candidates) == 0 {
		slog.Info("Gemini returned empty candidates list")
		return nil, errors.New("no response generated")
	}
	
	if len(resp.Candidates[0].Content.Parts) == 0 {
		slog.Info("Gemini returned candidate with empty parts list")
		return nil, errors.New("no response generated")
	}

	// Extract the response text
	responsePart := resp.Candidates[0].Content.Parts[0]
	slog.Info("Extracted first response part", "partType", fmt.Sprintf("%T", responsePart))
	
	responseText, ok := responsePart.(genai.Text)
	if !ok {
		slog.Info("Unexpected response part type", "type", fmt.Sprintf("%T", responsePart))
		return nil, fmt.Errorf("unexpected response type: %T", responsePart)
	}
	slog.Info("Response text extracted", "textLength", len(string(responseText)))
	
	// Clean up response - remove any markdown code block formatting
	cleanResponse := strings.TrimSpace(string(responseText))
	slog.Info("Trimmed response space", "beforeLength", len(string(responseText)), "afterLength", len(cleanResponse))
	
	cleanResponse = strings.TrimPrefix(cleanResponse, "```json")
	cleanResponse = strings.TrimPrefix(cleanResponse, "```")
	cleanResponse = strings.TrimSuffix(cleanResponse, "```")
	cleanResponse = strings.TrimSpace(cleanResponse)
	slog.Info("Cleaned response from markdown formatting", "finalLength", len(cleanResponse))

	// Validate the response is valid JSON
	var jsonResponse interface{}
	if err := json.Unmarshal([]byte(cleanResponse), &jsonResponse); err != nil {
		slog.Info("Invalid JSON response from LLM", "error", err, "response", cleanResponse)
		return nil, fmt.Errorf("invalid JSON response from LLM: %w", err)
	}
	slog.Info("Validated JSON response", "type", fmt.Sprintf("%T", jsonResponse))

	return []byte(cleanResponse), nil
}

// SimplePDFExtractor extracts text from a PDF file

// SimplePDFExtractor uploads a PDF file to the LlamaParse API and retrieves the parsed result once the job is completed.
func SimplePDFExtractor(filePath string) (string, error) {
	// Log the start of the file upload process
	slog.Info("Starting PDF file upload", "filePath", filePath)

	// Step 1: Upload the file to the LlamaParse API
	jobID, err := uploadFile(filePath)
	if err != nil {
		slog.Error("Failed to upload file", "filePath", filePath, "error", err)
		return "", fmt.Errorf("failed to upload file: %w", err)
	}
	// TODO: hardcoded for testing
	// var jobID="1c257b73-341f-439e-9271-90eed60a9415\"
	// var jobID="f4a8b15e-62c0-4ff3-8618-1d4a0356ea73"
	// Log successful file upload with job ID
	slog.Info("File uploaded successfully", "filePath", filePath, "jobID", jobID)

	// Step 2: Check the status of the parsing job repeatedly until it is "completed"
	var status string
	for {
    // Log the current job status check attempt
		slog.Info("Checking parsing job status", "jobID", jobID)

		status, err := checkJobStatus(jobID)
		if err != nil {
			slog.Error("Failed to check job status", "jobID", jobID, "error", err)
			return "", fmt.Errorf("failed to check job status: %w", err)
		}

	// 	// Log the retrieved job status
		slog.Info("Parsing job status retrieved", "jobID", jobID, "status", status)

		// If the job is completed or any other non-pending status, break the loop
		if status != "PENDING" {
			slog.Info("Parsing job is not pending, breaking out of the loop", "jobID", jobID, "status", status)
			break
		}

		// If the job is still pending, log the status and wait before retrying
		slog.Warn("Parsing job not completed yet", "jobID", jobID, "status", status)
		time.Sleep(5 * time.Second) // Retry every 5 seconds
	}
	slog.Info("Final parsing job status", "jobID", jobID, "status", status)


	// Step 3: Retrieve the result once the job is completed
	slog.Info("Retrieving parsing result", "jobID", jobID)

	result, err := getParsingResult(jobID)
	if err != nil {
		slog.Error("Failed to retrieve parsing result", "jobID", jobID, "error", err)
		return "", fmt.Errorf("failed to retrieve parsing result: %w", err)
	}

	// Log successful retrieval of the result
	slog.Info("Parsing result successfully retrieved", "jobID", jobID)

	// Return the parsing result (Markdown format)
	return result, nil
}

// uploadFile uploads a PDF file to the LlamaParse API and returns the job ID.
// Struct to match the JSON response from the API
type UploadResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

func uploadFile(filePath string) (string, error) {
	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		slog.Error("Failed to open file", "filePath", filePath, "error", err)
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Prepare the multipart form data
	var b bytes.Buffer
	writer := multipart.NewWriter(&b)
	part, err := writer.CreateFormFile("file", file.Name())
	if err != nil {
		slog.Error("Failed to create form file", "filePath", filePath, "error", err)
		return "", fmt.Errorf("failed to create form file: %w", err)
	}

	// Copy the file content into the form data
	_, err = io.Copy(part, file)
	if err != nil {
		slog.Error("Failed to copy file content", "filePath", filePath, "error", err)
		return "", fmt.Errorf("failed to read file content: %w", err)
	}

	// Close the writer to finalize the multipart form
	err = writer.Close()
	if err != nil {
		slog.Error("Failed to close multipart writer", "filePath", filePath, "error", err)
		return "", fmt.Errorf("failed to close writer: %w", err)
	}

	// Make the POST request to upload the file
	url := "https://api.cloud.llamaindex.ai/api/v1/parsing/upload"
	req, err := http.NewRequest("POST", url, &b)
	if err != nil {
		slog.Error("Failed to create request", "url", url, "error", err)
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", llamaCloudAPIKey))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("Failed to send request", "url", url, "error", err)
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Log the response status code for debugging purposes
	slog.Info("Received response from LlamaParse API", "statusCode", resp.StatusCode)

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("Failed to read response body", "error", err)
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	// Log the raw response body (useful for debugging)
	slog.Debug("Response body", "body", string(body))

	// Unmarshal the JSON response into the UploadResponse struct
	var response UploadResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		slog.Error("Failed to parse JSON response", "error", err)
		return "", fmt.Errorf("failed to parse JSON response: %w", err)
	}

	// Log the parsed jobID and status
	slog.Info("File uploaded successfully", "jobID", response.ID, "status", response.Status)

	// Return the job ID from the parsed response
	return response.ID, nil
}

// checkJobStatus checks the status of the parsing job using the job ID.
func checkJobStatus(jobID string) (string, error) {
	// Construct the URL with the job ID in the endpoint
	slog.Info("Checking job status", "jobID", jobID)
	url := fmt.Sprintf("https://api.cloud.llamaindex.ai/api/v1/parsing/job/%s/details", jobID)
	method := "GET"

	// Prepare the HTTP request
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		slog.Error("Failed to create request", "url", url, "error", err)
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set necessary headers
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", llamaCloudAPIKey))

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("Failed to send request", "url", url, "error", err)
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Log the response status code for debugging purposes
	slog.Info("Received response from LlamaParse API", "checking status",resp)

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("Failed to read response body", "error", err)
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	// Log the response body (you may want to change this to better handle large responses)
	slog.Info("Job status response", "body", string(body))

	// Return the job details (for simplicity, assuming it's a plain text or JSON response)
	return string(body), nil
}

// getParsingResult retrieves the text result of the parsing job using the provided job ID.
func getParsingResult(jobID string) (string, error) {
	// Construct the URL to fetch the text result
	url := fmt.Sprintf("https://api.cloud.llamaindex.ai/api/v1/parsing/job/%s/result/text", jobID)
	method := "GET"

	// Prepare the HTTP request
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		slog.Error("Failed to create request", "url", url, "error", err)
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set necessary headers
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", llamaCloudAPIKey))

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("Failed to send request", "url", url, "error", err)
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Log the response status code for debugging purposes
	slog.Info("Received response from LlamaParse API", "statusCode", resp.StatusCode)

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("Failed to read response body", "error", err)
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	// Log the response body (you may want to change this to better handle large responses)
	slog.Info("Job result response", "body", string(body))

	// Return the text content from the response
	return string(body), nil
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
		slog.Info("tempFile-nakul", "tempFile", tempFile.Name())
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
func  ParseDocumentWithTracking(ctx context.Context, payload []byte, jobID int) (Result, error) {
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

		slog.Info("Structured data nakul 69696969", "structuredData", structuredData)

		//update the structured data in the database
		updateQuery := "UPDATE jobs SET response = $1 WHERE id = $2"
		// Here we assume that documentID corresponds to the job id.
		_, err = db.Clients.DB.Exec(updateQuery, string(structuredData), jobID)
		if err != nil {
			finalErr = fmt.Errorf("failed to update job response: %w", err)
			tracker.UpdateStatus(documentID, StatusFailed, finalErr)
			slog.Info("Failed to update job response", "documentID", documentID, "attempt", attempt, "error", finalErr)
			continue // Try again if retries are available
		}


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
func ParseDocumentHandler(ctx context.Context, payload []byte, jobID int) (Result, error) {
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
		slog.Info("Garvit rand 696969",parsedPayload)
		parsedPayload["documentID"] = documentID
		parsedPayload["documentType"] = "url"
		parsedPayload["documentSource"]=parsedPayload["pdf_source"]
		parsedPayload["description"]=parsedPayload["description"]
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
	result, err := ParseDocumentWithTracking(ctx, payload, jobID)
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