// this module takes the user description and create the prompt for image generation
// it takes the random attributes from the json file

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"strings"
	"time"
)

// PromptData represents the structure of the JSON file containing prompt components
type PromptData struct {
	BasePrompt 	string   `json:"base_prompt"`
	SignTexts  	[]string `json:"sign_texts"`
	Emotions   	[]string `json:"emotions"`
	Backgrounds []string `json:"backgrounds"`
	Actions   	[]string `json:"actions"`
}

var promptData *PromptData
var rng *rand.Rand

// init initializes the random number generator and loads prompt data
func init() {
	// Initialize random number generator with current time as seed
	rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	
	// Load prompt data from JSON file
	loadPromptData()
}

// loadPromptData loads the prompt components from the JSON file
func loadPromptData() {
	data, err := ioutil.ReadFile("prompt_data.json")
	if err != nil {
		log.Fatalf("Error reading prompt_data.json: %v", err)
	}

	promptData = &PromptData{}
	err = json.Unmarshal(data, promptData)
	if err != nil {
		log.Fatalf("Error parsing prompt_data.json: %v", err)
	}

	log.Printf("Loaded prompt data: %d backgrounds, %d emotions, %d actions, %d sign texts", 
		len(promptData.Backgrounds), len(promptData.Emotions), len(promptData.Actions), len(promptData.SignTexts))
}

// getRandomBackground returns a random background from the list
func getRandomBackground() string {
	if len(promptData.Backgrounds) == 0 {
		return "neutral background"
	}
	return promptData.Backgrounds[rng.Intn(len(promptData.Backgrounds))]
}

// getRandomEmotion returns a random emotion from the list
func getRandomEmotion() string {
	if len(promptData.Emotions) == 0 {
		return "neutral"
	}
	return promptData.Emotions[rng.Intn(len(promptData.Emotions))]
}

// getRandomAction returns a random action from the list
func getRandomAction() string {
	if len(promptData.Actions) == 0 {
		return "standing normally"
	}
	return promptData.Actions[rng.Intn(len(promptData.Actions))]
}

// getRandomSignText returns a random sign text from the list
func getRandomSignText() string {
	if len(promptData.SignTexts) == 0 {
		return "hello"
	}
	return promptData.SignTexts[rng.Intn(len(promptData.SignTexts))]
}

// shouldUseSign determines if the subject should hold a sign (30% chance) or perform an action (70% chance)
func shouldUseSign() bool {
	return rng.Intn(100) < 30 // 30% chance for sign, 70% for action
}

// shouldBeGolden determines if this should be a special golden generation (2% chance)
func shouldBeGolden() bool {
	return rng.Intn(100) < 2 // 2% chance for golden special
}

// getActionOrSign returns either an action or a sign text based on random selection
func getActionOrSign() string {
	if shouldUseSign() {
		signText := getRandomSignText()
		return fmt.Sprintf("holding a sign that says \"%s\"", signText)
	}
	return getRandomAction()
}

// getGoldenSpecial returns the golden special text if applicable
func getGoldenSpecial() string {
	if shouldBeGolden() {
		return "\n\nThis is a special generation, make it all golden like its something rare."
	}
	return ""
}

// createPrompt creates the complete prompt by combining user description with random system specifications
func createPrompt(userDescription string) string {
	// Get random system specifications
	background := getRandomBackground()
	emotion := getRandomEmotion()
	actionOrSign := getActionOrSign()
	goldenSpecial := getGoldenSpecial()

	// Create the complete prompt by replacing placeholders
	prompt := promptData.BasePrompt
	prompt = strings.ReplaceAll(prompt, "{USER_DESCRIPTION}", userDescription)
	prompt = strings.ReplaceAll(prompt, "{BACKGROUND}", background)
	prompt = strings.ReplaceAll(prompt, "{EMOTION}", emotion)
	prompt = strings.ReplaceAll(prompt, "{ACTION_OR_SIGN}", actionOrSign)
	prompt = strings.ReplaceAll(prompt, "{GOLDEN_SPECIAL}", goldenSpecial)

	log.Printf("Generated prompt: Background=%s, Emotion=%s, Action/Sign=%s, Golden=%t", 
	background, emotion, actionOrSign, goldenSpecial != "")

	return prompt
}
