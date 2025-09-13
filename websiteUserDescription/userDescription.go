package main

import (
	"log"
	"time"

	"websiteUserDescription/config"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

// UserDescriptionItem represents the structure of the DynamoDB item
type UserDescriptionItem struct {
	UserID      string `json:"userId" dynamodbav:"userId"`
	Description string `json:"description" dynamodbav:"description"`
	LastUpdated string `json:"lastUpdated" dynamodbav:"lastUpdated"`
}

// getUserDescription retrieves user description from DynamoDB
func getUserDescription(userID string) UserDescriptionResponse {
	log.Printf("Getting description for user ID: %s", userID)

	// Get AWS configuration
	awsSecrets := config.GetAWSSecrets()

	// Create AWS session
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(awsSecrets.Region),
		Credentials: credentials.NewStaticCredentials(
			awsSecrets.AccessKeyID,
			awsSecrets.SecretAccessKey,
			"",
		),
	})
	if err != nil {
		log.Printf("Error creating AWS session: %v", err)
		// Return mock data as fallback
		return UserDescriptionResponse{
			UserID:      userID,
			Description: "Unable to retrieve description from database. Please try again later.",
			LastUpdated: time.Now().Format("2006-01-02 15:04:05"),
		}
	}

	// Create DynamoDB client
	svc := dynamodb.New(sess)

	// Prepare the GetItem input
	input := &dynamodb.GetItemInput{
		TableName: aws.String("UserDescription"),
		Key: map[string]*dynamodb.AttributeValue{
			"userId": {
				S: aws.String(userID),
			},
		},
	}

	// Execute the GetItem operation
	result, err := svc.GetItem(input)
	if err != nil {
		log.Printf("Error getting item from DynamoDB: %v", err)
		// Return mock data as fallback
		return UserDescriptionResponse{
			UserID:      userID,
			Description: "Unable to retrieve description from database. Please try again later.",
			LastUpdated: time.Now().Format("2006-01-02 15:04:05"),
		}
	}

	// Check if item was found
	if result.Item == nil {
		log.Printf("No description found for user ID: %s", userID)
		// Return empty description for new users
		return UserDescriptionResponse{
			UserID:      userID,
			Description: "No description found. Please add your first description below.",
			LastUpdated: "Never",
		}
	}

	// Unmarshal the result into our struct
	var item UserDescriptionItem
	err = dynamodbattribute.UnmarshalMap(result.Item, &item)
	if err != nil {
		log.Printf("Error unmarshaling DynamoDB item: %v", err)
		// Return mock data as fallback
		return UserDescriptionResponse{
			UserID:      userID,
			Description: "Error reading description from database. Please try again later.",
			LastUpdated: time.Now().Format("2006-01-02 15:04:05"),
		}
	}

	log.Printf("Successfully retrieved description for user ID: %s", userID)
	return UserDescriptionResponse(item)
}

// storeUserDescription saves user description to DynamoDB
func storeUserDescription(userID, description string) bool {
	log.Printf("Storing description for user ID: %s", userID)

	// Get AWS configuration
	awsSecrets := config.GetAWSSecrets()

	// Create AWS session
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(awsSecrets.Region),
		Credentials: credentials.NewStaticCredentials(
			awsSecrets.AccessKeyID,
			awsSecrets.SecretAccessKey,
			"",
		),
	})
	if err != nil {
		log.Printf("Error creating AWS session: %v", err)
		return false
	}

	// Create DynamoDB client
	svc := dynamodb.New(sess)

	// Create the item to store
	item := UserDescriptionItem{
		UserID:      userID,
		Description: description,
		LastUpdated: time.Now().Format("2006-01-02 15:04:05"),
	}

	// Marshal the item
	av, err := dynamodbattribute.MarshalMap(item)
	if err != nil {
		log.Printf("Error marshaling item: %v", err)
		return false
	}

	// Prepare the PutItem input
	input := &dynamodb.PutItemInput{
		TableName: aws.String("UserDescription"),
		Item:      av,
	}

	// Execute the PutItem operation
	_, err = svc.PutItem(input)
	if err != nil {
		log.Printf("Error putting item to DynamoDB: %v", err)
		return false
	}

	log.Printf("Successfully stored description for user ID: %s", userID)
	return true
}
