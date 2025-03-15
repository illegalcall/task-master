package supabase

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/supabase-community/gotrue-go"
)

var authClient gotrue.Client

// extractProjectRef extracts just the project reference ID from a Supabase URL
// From: akrqbuajqkirdekonpzy.supabase.co
// To: akrqbuajqkirdekonpzy
func extractProjectRef(url string) string {
	// Remove any protocol prefix
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")

	// Split by the first dot to get just the project reference
	parts := strings.Split(url, ".")
	return parts[0]
}

// InitClient initializes the Supabase authentication client
func InitClient(supabaseURL, supabaseKey string) error {
	// Extract just the project reference ID
	projectRef := extractProjectRef(supabaseURL)

	log.Printf("Initializing Supabase client with project reference: %s", projectRef)

	// Truncate key for logging to avoid exposing the full key
	truncatedKey := ""
	if len(supabaseKey) > 10 {
		truncatedKey = supabaseKey[:10] + "..."
	}
	log.Printf("Using API key: %s", truncatedKey)

	// Create auth client using just the project reference and API key
	client := gotrue.New(projectRef, supabaseKey)
	authClient = client

	// Test the connection
	log.Printf("Testing Supabase connection...")
	_, err := client.GetSettings()
	if err != nil {
		log.Printf("Supabase connection test failed: %v", err)
		return fmt.Errorf("failed to connect to Supabase: %w", err)
	}

	log.Printf("Supabase connection successful")
	return nil
}

// GetClient returns the initialized Supabase authentication client
func GetClient() gotrue.Client {
	if authClient == nil {
		// Initialize with environment variables as fallback
		url := os.Getenv("SUPABASE_URL")
		key := os.Getenv("SUPABASE_SERVICE_KEY") // Use service key for admin operations

		if url == "" || key == "" {
			panic("SUPABASE_URL and SUPABASE_SERVICE_KEY environment variables must be set")
		}

		// Extract just the project reference ID
		projectRef := extractProjectRef(url)

		client := gotrue.New(projectRef, key)
		authClient = client
	}
	return authClient
}

// ValidateCredentials checks if the provided credentials are valid
func ValidateCredentials(email, password string) (bool, error) {
	client := GetClient()

	log.Printf("Attempting authentication for email: %s", email)

	// Try to sign in with email and password
	res, err := client.SignInWithEmailPassword(email, password)
	if err != nil {
		log.Printf("Authentication error: %v", err)
		return false, fmt.Errorf("authentication failed: %w", err)
	}

	// If we successfully got a response, the credentials are valid
	isValid := res != nil && res.AccessToken != ""
	log.Printf("Authentication result for %s: %v", email, isValid)
	return isValid, nil
}
