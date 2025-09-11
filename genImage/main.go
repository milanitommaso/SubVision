package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	fmt.Println("Starting SubVision Image Generation Service...")
	
	// Set up logging
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	
	// Start SQS message processing in a separate goroutine
	go ProcessSQSMessages()
	
	fmt.Println("Service started. Press CTRL+C to exit")
	
	// Wait for termination signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	
	fmt.Println("Shutting down...")
}
