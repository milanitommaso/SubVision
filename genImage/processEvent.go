// this module checks if there a new message in the queue, if so check for the user description
// if there is the user description it call the generateImage module to generate the image
// it also handle the error,log it and put in the dlq

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"genImage/config"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
)

// Event represents the event data within the MessagePayload
type Event struct {
	EventType string `json:"event_type"`
	UserTier  string `json:"user_tier"`
	Months    int    `json:"months"`
	NBits     *int   `json:"n_bits"` // Using pointer to handle null values
}

// MessagePayload represents the expected structure of messages from the SQS queue
type MessagePayload struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	Datetime string `json:"datetime"`
	Event    Event  `json:"event"`
}

// ImageReadyEvent represents the structure of the imageReady event
type ImageReadyEvent struct {
	Username  string `json:"username"`
	ImagePath string `json:"image_path"`
}

// ProcessSQSMessages polls the SQS queue for messages and processes them
func ProcessSQSMessages() {
	// Get AWS secrets
	awsSecrets := config.GetAWSSecrets()

	// Initialize AWS session
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String(awsSecrets.Region),
		Credentials: credentials.NewStaticCredentials(awsSecrets.AccessKeyID, awsSecrets.SecretAccessKey, ""),
	})

	if err != nil {
		log.Printf("Error creating AWS session: %v", err)
		return
	}

	// Create SQS service client
	sqsClient := sqs.New(sess)

	// Set up receive message input
	receiveParams := &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(awsSecrets.SubsToProcessSqsQueueURL),
		MaxNumberOfMessages: aws.Int64(10),
		WaitTimeSeconds:     aws.Int64(20), // Long polling
		VisibilityTimeout:   aws.Int64(60), // 60 seconds to process the message
	}

	log.Println("Starting to poll SQS queue for messages...")

	// Poll for messages
	for {
		result, err := sqsClient.ReceiveMessage(receiveParams)
		if err != nil {
			log.Printf("Error receiving message from SQS: %v", err)
			// message to telegram
			time.Sleep(5 * time.Second) // Wait before retrying
			continue
		}

		// Process received messages
		for _, message := range result.Messages {
			processMessage(sqsClient, message, awsSecrets)
		}

		// Small delay to prevent excessive polling
		time.Sleep(500 * time.Millisecond)
	}
}

// processMessage handles a single SQS message
func processMessage(sqsClient *sqs.SQS, message *sqs.Message, awsSecrets config.AWSSecrets) {
	log.Printf("Processing message: %s", *message.MessageId)

	// Parse message body
	var payload MessagePayload
	err := json.Unmarshal([]byte(*message.Body), &payload)
	if err != nil {
		log.Printf("Error parsing message body: %v", err)
		moveMessageToDLQ(sqsClient, message, awsSecrets, "Failed to parse message body")
		return
	}

	// Process based on user ID
	if payload.UserID <= 0 {
		log.Printf("Message missing or invalid UserID: %s", *message.MessageId)
		moveMessageToDLQ(sqsClient, message, awsSecrets, "Missing or invalid UserID")
		return
	}

	// Get user description (this would call your user description module)
	userDescription, err := GetUserDescription(payload.UserID)
	if err != nil {
		log.Printf("Failed to get user description: %v", err)
		moveMessageToDLQ(sqsClient, message, awsSecrets, "Failed to get user description")
		return
	}

	deleteParams := &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(awsSecrets.SubsToProcessSqsQueueURL),
		ReceiptHandle: message.ReceiptHandle,
	}

	// Return if there is no user description
	if userDescription == "" {
		log.Printf("No description found for user ID: %d", payload.UserID)

		_, err = sqsClient.DeleteMessage(deleteParams)
		if err != nil {
			log.Printf("Error deleting message: %v", err)
		}

		return
	}

	prompt := createPrompt(userDescription)

	// Generate image by calling the GenerateImage module
	imagePath, err := GenerateImage(prompt, payload.Username)
	if err != nil {
		log.Printf("Failed to generate image: %v", err)
		moveMessageToDLQ(sqsClient, message, awsSecrets, "Failed to generate image")
		return
	}

	log.Printf("Image successfully generated and saved to: %s", imagePath)

	// Send imageReady event to ReadyImages.fifo SQS queue
	err = sendImageReadyEvent(sqsClient, awsSecrets, payload, imagePath)
	if err != nil {
		log.Printf("Failed to send imageReady event: %v", err)
		// Note: We don't return here as the image was successfully generated
		// The failure to send the event shouldn't prevent further processing
	}

	// Delete message from the queue after successful processing
	_, err = sqsClient.DeleteMessage(deleteParams)
	if err != nil {
		log.Printf("Error deleting message: %v", err)
	}

	// Send image to Discord (this would call your sendImageDiscord module)
	// err = SendImageToDiscord(imagePath)
	// if err != nil {
	//     log.Printf("Failed to send image to Discord: %v", err)
	//     send message to telegram
	//     return
	// }

	// For now, we'll just log that we would process this message
	log.Printf("Would process message for UserID: %d", payload.UserID)
}

// moveMessageToDLQ moves a failed message to the dead letter queue
func moveMessageToDLQ(sqsClient *sqs.SQS, message *sqs.Message, awsSecrets config.AWSSecrets, reason string) {
	// send message to telegram

	messageGroupID := "0"
	// Create deduplication ID to prevent duplicate messages
	deduplicationID := fmt.Sprintf("%d", time.Now().Unix())

	// Create a new message for the DLQ
	dlqMessage := *message.Body
	dlqParams := &sqs.SendMessageInput{
		MessageBody:  &dlqMessage,
		QueueUrl:     aws.String(awsSecrets.SubsToProcessSqsDlqQueueURL),
		MessageGroupId:         aws.String(messageGroupID),
		MessageDeduplicationId: aws.String(deduplicationID),
	}

	// Add failure reason as message attribute
	dlqParams.MessageAttributes = map[string]*sqs.MessageAttributeValue{
		"FailureReason": {
			DataType:    aws.String("String"),
			StringValue: aws.String(reason),
		},
		"OriginalMessageID": {
			DataType:    aws.String("String"),
			StringValue: message.MessageId,
		},
		
	}

	// Send to DLQ
	_, err := sqsClient.SendMessage(dlqParams)
	if err != nil {
		log.Printf("Error sending message to DLQ: %v", err)
	} else {
		log.Printf("Message sent to DLQ: %s, Reason: %s", *message.MessageId, reason)
	}

	// Delete the original message from the source queue
	deleteParams := &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(awsSecrets.SubsToProcessSqsQueueURL),
		ReceiptHandle: message.ReceiptHandle,
	}

	_, err = sqsClient.DeleteMessage(deleteParams)
	if err != nil {
		log.Printf("Error deleting message after moving to DLQ: %v", err)
	}
}

// sendImageReadyEvent sends an imageReady event to the ReadyImages.fifo SQS queue
func sendImageReadyEvent(sqsClient *sqs.SQS, awsSecrets config.AWSSecrets, payload MessagePayload, imagePath string) error {
	// Create the imageReady event
	imageReadyEvent := ImageReadyEvent{
		Username:      payload.Username,
		ImagePath:     imagePath,
	}

	// Marshal the event to JSON
	eventJSON, err := json.Marshal(imageReadyEvent)
	if err != nil {
		return fmt.Errorf("failed to marshal imageReady event: %v", err)
	}

	messageGroupID := "0"
	
	// Create deduplication ID to prevent duplicate messages
	deduplicationID := fmt.Sprintf("%d_%s_%d", payload.UserID, payload.Event.EventType, time.Now().Unix())

	// Send message to ReadyImages.fifo queue
	sendParams := &sqs.SendMessageInput{
		MessageBody:            aws.String(string(eventJSON)),
		QueueUrl:               aws.String(awsSecrets.ReadyImagesSqsQueueURL),
		MessageGroupId:         aws.String(messageGroupID),
		MessageDeduplicationId: aws.String(deduplicationID),
	}

	_, err = sqsClient.SendMessage(sendParams)
	if err != nil {
		return fmt.Errorf("failed to send imageReady event to SQS: %v", err)
	}

	log.Printf("Successfully sent imageReady event to ReadyImages.fifo queue for user %d", payload.UserID)
	return nil
}
