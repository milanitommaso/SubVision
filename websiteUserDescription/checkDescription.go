package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
	
	"websiteUserDescription/config"

	"google.golang.org/genai"
)

const (
	// Model configuration
	geminiModel = "gemini-2.5-flash-lite"
	
	// Timeout for API calls
	apiTimeout = 30 * time.Second
)

// SafetyPrompt represents the structure of the safety prompt JSON file
type SafetyPrompt struct {
	SystemPrompt        string   `json:"system_prompt"`
	UserPromptTemplate  string   `json:"user_prompt_template"`
	ExpectedResponses   []string `json:"expected_responses"`
}

// loadSafetyPrompt loads the safety prompt configuration from JSON file
func loadSafetyPrompt() (*SafetyPrompt, error) {
	data, err := os.ReadFile("safety_prompt.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read safety prompt file: %w", err)
	}
	
	var prompt SafetyPrompt
	if err := json.Unmarshal(data, &prompt); err != nil {
		return nil, fmt.Errorf("failed to parse safety prompt JSON: %w", err)
	}
	
	return &prompt, nil
}

// checkDescriptionWithLLM uses Gemini 2.5 Flash Lite to validate user descriptions.
// Returns true if the description is safe for work and doesn't contain prompt injection.
func checkDescriptionWithLLM(description string) bool {
	if strings.TrimSpace(description) == "" {
		log.Printf("Empty description provided")
		return false
	}

	promptConfig, err := loadSafetyPrompt()
	if err != nil {
		log.Printf("Error loading safety prompt: %v", err)
		return false
	}

	response, err := queryLLM(description, promptConfig)
	if err != nil {
		log.Printf("Error querying LLM: %v", err)
		return false
	}

	return validateResponse(response, promptConfig.ExpectedResponses)
}

// queryLLM handles the LLM API interaction
func queryLLM(description string, promptConfig *SafetyPrompt) (string, error) {
	googleAPISecrets := config.GetGoogleAPISecrets()
	
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
	defer cancel()

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  googleAPISecrets.APIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create Gemini client: %w", err)
	}

	fullPrompt := buildPrompt(description, promptConfig)

	result, err := client.Models.GenerateContent(
		ctx,
		geminiModel,
		genai.Text(fullPrompt),
		nil,
	)
	if err != nil {
		return "", fmt.Errorf("failed to generate content: %w", err)
	}

	if len(result.Candidates) == 0 {
		return "", fmt.Errorf("no candidates returned from Gemini API")
	}

	return extractResponse(result.Candidates[0].Content.Parts), nil
}

// buildPrompt constructs the full prompt for the LLM
func buildPrompt(description string, promptConfig *SafetyPrompt) string {
	userPrompt := strings.ReplaceAll(promptConfig.UserPromptTemplate, "{description}", description)
	return fmt.Sprintf("%s\n\n%s", promptConfig.SystemPrompt, userPrompt)
}

// extractResponse extracts the text response from LLM parts
func extractResponse(parts []*genai.Part) string {
	for _, part := range parts {
		if part.Text != "" {
			return strings.TrimSpace(strings.ToLower(part.Text))
		}
	}
	return ""
}

// validateResponse checks if the LLM response is valid and safe
func validateResponse(response string, expectedResponses []string) bool {
	// Normalize response
	normalizedResponse := strings.TrimSpace(strings.ToLower(response))
	
	// Check if response matches expected safe response
	for _, expected := range expectedResponses {
		if normalizedResponse == strings.ToLower(expected) {
			return normalizedResponse == "yes"
		}
	}
	
	// Log unexpected response and default to unsafe
	log.Printf("Unexpected LLM response: '%s', expected one of: %v", response, expectedResponses)
	return false
}
