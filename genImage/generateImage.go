// this module uses Google Gemini API to generate images and save them to local disk
// the function GenerateImage returns the image path, in case of error an error is also returned

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
	
	"genImage/config"

	"google.golang.org/genai"
)

// GenerateImage creates an image using the Google Gemini API based on the provided prompt
// and saves it to disk. Returns the path to the saved image or an error.
func GenerateImage(prompt string, username string) (string, error) {
	googleAPISecrets := config.GetGoogleAPISecrets()

	ctx := context.Background()
	
	// Create Gemini client
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
        APIKey:  googleAPISecrets.APIKey,
        Backend: genai.BackendGeminiAPI,
    })
	if err != nil {
		return "", fmt.Errorf("failed to create Gemini client: %w", err)
	}

	// Generate content using Gemini 2.5 Flash Image Preview model
	result, err := client.Models.GenerateContent(
		ctx,
		"gemini-2.5-flash-image-preview",
		genai.Text(prompt),
		nil,
	)
	if err != nil {
		return "", fmt.Errorf("failed to generate content: %w", err)
	}

	// Check if we have candidates
	if len(result.Candidates) == 0 {
		return "", fmt.Errorf("no candidates returned from Gemini API")
	}

	// Look for image data in the response
	var imageBytes []byte
	for _, part := range result.Candidates[0].Content.Parts {
		if part.Text != "" {
			// Log any text response for debugging
			log.Printf("Gemini text response: %s", part.Text)
		} else if part.InlineData != nil {
			imageBytes = part.InlineData.Data
			break
		}
	}

	if len(imageBytes) == 0 {
		return "", fmt.Errorf("no image data found in Gemini response")
	}

	// Create output directory if it doesn't exist
	outputDir := "../websiteOverlay/output_images"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate unique filename using timestamp
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("img_%s_%s.png", username, timestamp)
	path := filepath.Join(outputDir, filename)

	// Save image to file
	if err := os.WriteFile(path, imageBytes, 0644); err != nil {
		return "", fmt.Errorf("failed to save image to file: %w", err)
	}

	return filename, nil
}
