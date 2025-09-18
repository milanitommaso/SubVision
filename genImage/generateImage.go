// this module uses runware API to generate images and save them to local disk
// the function GenerateImage returns the image path, in case of error an error is also returned

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"genImage/config"
	"github.com/google/uuid"
)

// ModelsConfig represents the structure of the models JSON file
type ModelsConfig struct {
	Models []string `json:"models"`
}

// getRandomModel loads models from the JSON file and returns a randomly selected one
func getRandomModel() (string, error) {
	file, err := os.Open("models.json")
	if err != nil {
		return "", fmt.Errorf("failed to open models.json: %w", err)
	}
	defer file.Close()

	var config ModelsConfig
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return "", fmt.Errorf("failed to decode models.json: %w", err)
	}

	if len(config.Models) == 0 {
		return "", fmt.Errorf("no models found in models.json")
	}

	// Seed the random number generator with current time
	rand.Seed(time.Now().UnixNano())

	// Select a random model
	randomIndex := rand.Intn(len(config.Models))
	return config.Models[randomIndex], nil
}

// GenerateImage creates an image using the Runware API based on the provided prompt
// and saves it to disk. Returns the path to the saved image or an error.
func GenerateImage(prompt string, username string) (string, error) {
	runwareSecrets := config.GetRunwareAPISecrets()

	// Get a random model
	randomModel, err := getRandomModel()
	if err != nil {
		return "", fmt.Errorf("failed to get random model: %w", err)
	}

	fmt.Printf("Using model: %s\n", randomModel)

	// Prepare request payload
	uuid := uuid.New()
	payload := []map[string]interface{}{
		{
			"taskType":       "imageInference",
			"taskUUID":       uuid,
			"positivePrompt": prompt,
			"model":          randomModel,
			"numberResults":  1,
		},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.runware.ai/v1", bytes.NewBuffer(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+runwareSecrets.APIKey)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call Runware API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("runware API error: %s", string(body))
	}

	var apiResp struct {
		Data []struct {
			ImageURL string `json:"imageURL"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return "", fmt.Errorf("failed to decode Runware response: %w", err)
	}

	if len(apiResp.Data) == 0 || apiResp.Data[0].ImageURL == "" {
		return "", fmt.Errorf("no image URL found in Runware response")
	}

	imageURL := apiResp.Data[0].ImageURL

	// Download the image
	imgResp, err := client.Get(imageURL)
	if err != nil {
		return "", fmt.Errorf("failed to download image: %w", err)
	}
	defer imgResp.Body.Close()
	if imgResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(imgResp.Body)
		return "", fmt.Errorf("failed to download image, status: %d, body: %s", imgResp.StatusCode, string(body))
	}

	// Create output directory if it doesn't exist
	outputDir := "../websiteOverlay/output_images"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate unique filename using timestamp
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("img_%s_%s.jpg", username, timestamp)
	path := filepath.Join(outputDir, filename)

	// Save the image as jpg
	outFile, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("failed to create image file: %w", err)
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, imgResp.Body); err != nil {
		return "", fmt.Errorf("failed to save image: %w", err)
	}

	return filename, nil
}
