// this module send the generated image to discord channel

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"genImage/config"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
)

// SendImageToDiscord sends the generated image to a Discord channel
func SendImageToDiscord(imagePath string, username string) {
	discordSecrets := config.GetDiscordSecrets()

	if discordSecrets.Token == "" || discordSecrets.ChannelId == "" {
		log.Printf("discord configuration is missing: token or channel ID not set")
		return
	}

	imagePath = fmt.Sprintf("../websiteOverlay/output_images/%s", imagePath)

	// Read the image file
	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		log.Printf("failed to read image file: %v", err)
		return
	}

	// Get just the filename without path
	baseFilename := filepath.Base(imagePath)

	// Create the JSON payload
	payload := map[string]interface{}{
		"content": fmt.Sprintf("Image generated for %s", username),
		"attachments": []map[string]interface{}{
			{
				"id":          0,
				"description": "Generated image",
				"filename":    baseFilename,
			},
		},
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		log.Printf("failed to marshal payload: %v", err)
		return
	}

	// Create a buffer to hold the multipart data
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add the JSON payload
	payloadPart, err := writer.CreateFormField("payload_json")
	if err != nil {
		log.Printf("failed to create payload field: %v", err)
		return
	}
	payloadPart.Write(payloadJSON)

	// Add the image file
	filePart, err := writer.CreateFormFile("files[0]", baseFilename)
	if err != nil {
		log.Printf("failed to create file field: %v", err)
		return
	}
	filePart.Write(imageData)

	// Close the multipart writer
	err = writer.Close()
	if err != nil {
		log.Printf("failed to close multipart writer: %v", err)
		return
	}

	// Create the Discord API URL
	apiURL := fmt.Sprintf("https://discord.com/api/v10/channels/%s/messages", discordSecrets.ChannelId)

	// Create HTTP request
	req, err := http.NewRequest("POST", apiURL, &buf)
	if err != nil {
		log.Printf("failed to create HTTP request: %v", err)
		return
	}

	// Set required headers
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", fmt.Sprintf("Bot %s", discordSecrets.Token))

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("failed to send HTTP request: %v", err)
		return
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("discord API returned status %d", resp.StatusCode)
		return
	}

	log.Printf("Successfully sent image to Discord channel")
}
