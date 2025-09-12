package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

type DescriptionRequest struct {
	UserID      string `json:"userId"`
	Description string `json:"description"`
}

type DescriptionResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Valid   bool   `json:"valid"`
}

type UserDataResponse struct {
	UserID      string `json:"userId"`
	Description string `json:"description"`
	LastUpdated string `json:"lastUpdated"`
}

func main() {
	// Serve static files
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static/"))))
	
	// Serve the main page
	http.HandleFunc("/", serveHomePage)
	
	// API endpoints
	http.HandleFunc("/api/user-data", getUserData)
	http.HandleFunc("/api/submit-description", submitDescription)
	
	fmt.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func serveHomePage(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./static/index.html")
}

func getUserData(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	userID := r.URL.Query().Get("userId")
	if userID == "" {
		http.Error(w, "User ID is required", http.StatusBadRequest)
		return
	}
	
	// For now, return mock data - in the future this will call getUserDescription()
	userData := getUserDescription(userID)
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(userData)
}

func submitDescription(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	var req DescriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	
	if req.UserID == "" || req.Description == "" {
		http.Error(w, "User ID and description are required", http.StatusBadRequest)
		return
	}
	
	// Check description with LLM (placeholder for now)
	isValid := checkDescriptionWithLLM(req.Description)
	
	var response DescriptionResponse
	if isValid {
		// Store in database (placeholder for now)
		success := storeUserDescription(req.UserID, req.Description)
		if success {
			response = DescriptionResponse{
				Success: true,
				Message: "Description accepted and saved successfully!",
				Valid:   true,
			}
		} else {
			response = DescriptionResponse{
				Success: false,
				Message: "Description was accepted but failed to save. Please try again.",
				Valid:   true,
			}
		}
	} else {
		response = DescriptionResponse{
			Success: false,
			Message: "Description was rejected by our validation system. Please provide a more appropriate description.",
			Valid:   false,
		}
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
