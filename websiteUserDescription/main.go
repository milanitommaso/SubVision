package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"websiteUserDescription/config"

	"github.com/clerk/clerk-sdk-go/v2"
	clerkhttp "github.com/clerk/clerk-sdk-go/v2/http"
	"github.com/clerk/clerk-sdk-go/v2/user"
)

type SetUserDescriptionRequest struct {
	Description string `json:"description"`
}

type SetUserDescriptionResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Valid   bool   `json:"valid"`
}

type UserDescriptionResponse struct {
	UserID      string `json:"userId"`
	Description string `json:"description"`
	LastUpdated string `json:"lastUpdated"`
}

type GetUserDataResponse struct {
    UserID      string `json:"userId"`
    Username    string `json:"username"`
	Description string `json:"description"`
	LastUpdated string `json:"lastUpdated"`
}

func main() {
	// Initialize Clerk client
	clerkSecrets := config.GetClerkSecrets()
	clerk.SetKey(clerkSecrets.SecretKey)

	mux := http.NewServeMux()

	// Serve static files
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static/"))))

	// Serve the main page
	mux.HandleFunc("/", serveHomePage)

	// API endpoints
	getUserDataHandler := http.HandlerFunc(getUserData)
	mux.Handle(
		"/api/user-data",
		clerkhttp.WithHeaderAuthorization()(getUserDataHandler),
	)

    submitDescriptionHandler := http.HandlerFunc(submitDescription)
    mux.Handle(
        "/api/submit-description",
        clerkhttp.WithHeaderAuthorization()(submitDescriptionHandler),
    )
	
	fmt.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func serveHomePage(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./static/index.html")
}

func getUserData(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	claims, ok := clerk.SessionClaimsFromContext(r.Context())
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"access": "unauthorized"}`))
		return
	}

	usr, err := user.Get(r.Context(), claims.Subject)
	if err != nil {
		// handle the error
	}

	var username string = *usr.ExternalAccounts[0].Username
	var twitchUserId string = usr.ExternalAccounts[0].ProviderUserID

	userData := getUserDescription(twitchUserId)

    userDataWithUsername := GetUserDataResponse{
        UserID:      userData.UserID,
        Username:    username,
        Description: userData.Description,
        LastUpdated: userData.LastUpdated,
    }
    
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(userDataWithUsername)
}

func submitDescription(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	var req SetUserDescriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Description == "" {
		http.Error(w, "Description is required", http.StatusBadRequest)
		return
	}
	
    claims, ok := clerk.SessionClaimsFromContext(r.Context())
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"access": "unauthorized"}`))
		return
	}

	usr, err := user.Get(r.Context(), claims.Subject)
	if err != nil {
		// handle the error
	}

	var twitchUserId string = usr.ExternalAccounts[0].ProviderUserID

	var response SetUserDescriptionResponse

	// check that description was not submitted too recently, if it was updated less than 10 seconds ago, reject
	lastUpdated := getLastUpdated(twitchUserId)
	parsedTime, err := time.ParseInLocation("2006-01-02 15:04:05", lastUpdated, time.Local)
	if err == nil && time.Since(parsedTime).Seconds() < 10 {
		response = SetUserDescriptionResponse{
			Success: false,
			Message: "You can only update your description once every 10 seconds. Please wait a moment and try again.",
			Valid:   false,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}
	
	// Check description with LLM (placeholder for now)
	isValid := checkDescriptionWithLLM(req.Description)
	if isValid {
		// Store in database (placeholder for now)
		success := storeUserDescription(twitchUserId, req.Description)
		if success {
			response = SetUserDescriptionResponse{
				Success: true,
				Message: "Description accepted and saved successfully!",
				Valid:   true,
			}
		} else {
			response = SetUserDescriptionResponse{
				Success: false,
				Message: "Description was accepted but failed to save. Please try again.",
				Valid:   true,
			}
		}
	} else {
		response = SetUserDescriptionResponse{
			Success: false,
			Message: "Description was rejected by our validation system. Please provide a more appropriate description.",
			Valid:   false,
		}
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
