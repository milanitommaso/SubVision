package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"websiteOverlay/config"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow connections from any origin
	},
	HandshakeTimeout: 45 * time.Second,
	ReadBufferSize:   1024,
	WriteBufferSize:  1024,
}

// Connected WebSocket clients with connection info
type ClientInfo struct {
	conn      *websocket.Conn
	lastPing  time.Time
	connected time.Time
}

var clients = make(map[*websocket.Conn]*ClientInfo)
var broadcast = make(chan []byte)

// Synchronization channels
var eventProcessing = make(chan bool, 1)
var eventAcknowledged = make(chan bool, 1)

// SQS message structure
type SQSMessage struct {
	MessageID string `json:"messageId"`
	Body      string `json:"body"`
	Timestamp string `json:"timestamp"`
}

// ImageReadyEvent structure from the queue
type ImageReadyEvent struct {
	Username  string `json:"username"`
	ImagePath string `json:"image_path"`
}

// Event structure to send to frontend
type Event struct {
	Type      string      `json:"type"`
	Data      interface{} `json:"data"`
	Timestamp string      `json:"timestamp"`
}

func main() {
	// Initialize AWS session
	awsSecrets := config.GetAWSSecrets()
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(awsSecrets.Region),
		Credentials: credentials.NewStaticCredentials(
			awsSecrets.AccessKeyID,
			awsSecrets.SecretAccessKey,
			"",
		),
	})
	if err != nil {
		log.Fatal("Failed to create AWS session:", err)
	}

	sqsClient := sqs.New(sess)

	// Start WebSocket message broadcaster
	go handleMessages()

	// Start connection cleanup routine
	go cleanupStaleConnections()

	// Start SQS polling in a separate goroutine
	go pollSQSQueue(sqsClient, awsSecrets.ReadyImagesSqsQueueURL)

	// HTTP routes
	http.HandleFunc("/", serveHome)
	http.HandleFunc("/ws", handleWebSocket)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static/"))))
	http.Handle("/output_images/", http.StripPrefix("/output_images/", http.FileServer(http.Dir("output_images/"))))

	fmt.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// Serve the main HTML page
func serveHome(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "static/index.html")
}

// Handle WebSocket connections
func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}
	defer conn.Close()

	// Register new client with connection info
	clientInfo := &ClientInfo{
		conn:      conn,
		lastPing:  time.Now(),
		connected: time.Now(),
	}
	clients[conn] = clientInfo
	log.Printf("New WebSocket client connected (total clients: %d)", len(clients))

	// Send welcome message
	welcomeEvent := Event{
		Type:      "connection",
		Data:      "Connected to SQS monitor",
		Timestamp: time.Now().Format(time.RFC3339),
	}
	welcomeJSON, _ := json.Marshal(welcomeEvent)
	conn.WriteMessage(websocket.TextMessage, welcomeJSON)

	// Listen for messages from client
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Println("WebSocket client disconnected:", err)
			delete(clients, conn)
			break
		}

		// Parse incoming message
		var clientMessage map[string]interface{}
		if err := json.Unmarshal(message, &clientMessage); err == nil {
			if msgType, ok := clientMessage["type"].(string); ok {
				switch msgType {
				case "event_acknowledged":
					log.Println("Event acknowledged by frontend")
					// Signal that the event has been acknowledged
					select {
					case eventAcknowledged <- true:
					default:
						// Channel is full, acknowledgment already received
					}
				case "ping":
					// Update last ping time for this client
					if clientInfo, exists := clients[conn]; exists {
						clientInfo.lastPing = time.Now()
					}
					// Respond to heartbeat ping with pong
					pongEvent := Event{
						Type:      "pong",
						Data:      "heartbeat response",
						Timestamp: time.Now().Format(time.RFC3339),
					}
					pongJSON, _ := json.Marshal(pongEvent)
					conn.WriteMessage(websocket.TextMessage, pongJSON)
					log.Println("Heartbeat pong sent to client")
				}
			}
		}
	}
}

// Handle broadcasting messages to all connected clients
func handleMessages() {
	for {
		msg := <-broadcast
		for conn, clientInfo := range clients {
			err := clientInfo.conn.WriteMessage(websocket.TextMessage, msg)
			if err != nil {
				log.Printf("WebSocket write error: %v", err)
				clientInfo.conn.Close()
				delete(clients, conn)
			}
		}
	}
}

// Poll SQS queue with synchronization
func pollSQSQueue(sqsClient *sqs.SQS, queueURL string) {
	log.Println("Starting SQS polling for queue:", queueURL)

	for {
		// Receive messages from SQS
		result, err := sqsClient.ReceiveMessage(&sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(queueURL),
			MaxNumberOfMessages: aws.Int64(1),  // Process one message at a time
			WaitTimeSeconds:     aws.Int64(20), // Long polling
			VisibilityTimeout:   aws.Int64(30),
		})

		if err != nil {
			log.Printf("Error receiving SQS messages: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		// Process each message (should be only one due to MaxNumberOfMessages: 1)
		for _, message := range result.Messages {
			log.Printf("Received SQS message: %s", *message.Body)
			log.Println("Stopping polling - processing event")

			// Signal that we're processing an event (stop polling)
			select {
			case eventProcessing <- true:
			default:
				// Channel is full, already processing
			}

			// Parse the ImageReadyEvent from the message body
			var imageEvent ImageReadyEvent
			var event Event

			if err := json.Unmarshal([]byte(*message.Body), &imageEvent); err != nil {
				log.Printf("Error parsing ImageReadyEvent: %v", err)
				// Continue with raw message if parsing fails
				event = Event{
					Type: "sqs_message",
					Data: map[string]interface{}{
						"messageId": *message.MessageId,
						"body":      *message.Body,
						"receipt":   *message.ReceiptHandle,
					},
					Timestamp: time.Now().Format(time.RFC3339),
				}
			} else {
				// Add the folder path to the image path
				imageEvent.ImagePath = "/output_images/" + imageEvent.ImagePath

				// Successfully parsed ImageReadyEvent
				log.Printf("Parsed ImageReadyEvent - Username: %s, ImagePath: %s", imageEvent.Username, imageEvent.ImagePath)

				// Create event for frontend with structured data
				event = Event{
					Type: "image_ready",
					Data: map[string]interface{}{
						"username":  imageEvent.Username,
						"imagePath": imageEvent.ImagePath,
						"messageId": *message.MessageId,
						"receipt":   *message.ReceiptHandle,
					},
					Timestamp: time.Now().Format(time.RFC3339),
				}
			}

			// Broadcast to all connected WebSocket clients
			eventJSON, err := json.Marshal(event)
			if err != nil {
				log.Printf("Error marshaling event: %v", err)
				continue
			}

			// Send to broadcast channel
			select {
			case broadcast <- eventJSON:
				log.Println("Event sent to frontend, waiting for acknowledgment...")
			default:
				log.Println("Broadcast channel full, dropping message")
				continue
			}

			// Wait for frontend acknowledgment with timeout
			acknowledged := false
			select {
			case <-eventAcknowledged:
				log.Println("Frontend acknowledged event, resuming polling")
				acknowledged = true
			case <-time.After(10 * time.Second):
				log.Println("Timeout waiting for frontend acknowledgment, resuming polling")
			}

			// Delete message from queue only if frontend acknowledged
			if acknowledged {
				_, err = sqsClient.DeleteMessage(&sqs.DeleteMessageInput{
					QueueUrl:      aws.String(queueURL),
					ReceiptHandle: message.ReceiptHandle,
				})
				if err != nil {
					log.Printf("Error deleting SQS message: %v", err)
				} else {
					log.Println("SQS message deleted after acknowledgment")
				}
			} else {
				log.Println("Message not deleted due to missing acknowledgment")
			}

			// Clear the processing signal
			select {
			case <-eventProcessing:
			default:
				// Channel is empty
			}

			log.Println("Resuming polling")
		}

		// Small delay between polling cycles if no messages
		if len(result.Messages) == 0 {
			time.Sleep(1 * time.Second)
		}
	}
}

// Clean up stale connections periodically
func cleanupStaleConnections() {
	ticker := time.NewTicker(60 * time.Second) // Check every minute
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		staleConnections := make([]*websocket.Conn, 0)

		// Find stale connections (no ping for more than 2 minutes)
		for conn, clientInfo := range clients {
			timeSinceLastPing := now.Sub(clientInfo.lastPing)
			if timeSinceLastPing > 2*time.Minute {
				log.Printf("Found stale connection (last ping: %v ago)", timeSinceLastPing)
				staleConnections = append(staleConnections, conn)
			}
		}

		// Clean up stale connections
		for _, conn := range staleConnections {
			if clientInfo, exists := clients[conn]; exists {
				log.Printf("Closing stale connection (connected: %v ago, last ping: %v ago)",
					now.Sub(clientInfo.connected), now.Sub(clientInfo.lastPing))
				clientInfo.conn.Close()
				delete(clients, conn)
			}
		}

		if len(staleConnections) > 0 {
			log.Printf("Cleaned up %d stale connections. Active connections: %d",
				len(staleConnections), len(clients))
		}
	}
}
