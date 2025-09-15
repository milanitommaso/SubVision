// this module is responsible to return the user description given the userId
// it calls aws dynamodb to get the user description

package main

import (
	"fmt"
	"log"
	"strconv"

	"genImage/config"

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
}

// GetUserDescription retrieves a user's description from DynamoDB based on their user ID
func GetUserDescription(userID int) (string, error) {
	log.Printf("Getting description for user ID: %d", userID)

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
		return "", fmt.Errorf("failed to create AWS session: %w", err)
	}

	// Create DynamoDB client
	svc := dynamodb.New(sess)

	// Convert userID to string for DynamoDB key
	userIDStr := strconv.Itoa(userID)

	// Prepare the GetItem input
	input := &dynamodb.GetItemInput{
		TableName: aws.String("UserDescription"),
		Key: map[string]*dynamodb.AttributeValue{
			"userId": {
				S: aws.String(userIDStr),
			},
		},
	}

	// Execute the GetItem operation
	result, err := svc.GetItem(input)
	if err != nil {
		log.Printf("Error getting item from DynamoDB: %v", err)
		return "", fmt.Errorf("failed to get item from DynamoDB: %w", err)
	}

	// Check if item was found
	if result.Item == nil {
		log.Printf("No description found for user ID: %d", userID)
		return "", nil
	}

	// Unmarshal the result into our struct
	var item UserDescriptionItem
	err = dynamodbattribute.UnmarshalMap(result.Item, &item)
	if err != nil {
		log.Printf("Error unmarshaling DynamoDB item: %v", err)
		return "", fmt.Errorf("failed to unmarshal DynamoDB item: %w", err)
	}

	log.Printf("Successfully retrieved description for user ID: %d", userID)
	return item.Description, nil
}
