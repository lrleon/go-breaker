package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"context"
	"github.com/opsgenie/opsgenie-go-sdk-v2/alert"
	"github.com/opsgenie/opsgenie-go-sdk-v2/client"
)

func main() {
	fmt.Println("🔑 OpsGenie Connection Tester")
	fmt.Println("==============================")
	fmt.Println("Note: You need an API key with FULL ACCESS permissions, not just team integration.")
	fmt.Println("      API keys can be created at https://app.opsgenie.com/settings/api-key-management")
	fmt.Println()

	// Verify that the API key exists
	apiKey := os.Getenv("OPSGENIE_API_KEY")
	if apiKey == "" {
		log.Fatal("❌ OPSGENIE_API_KEY environment variable is required")
	}
	fmt.Println("✓ API Key found in environment variables")

	// Check if verbose mode is enabled
	verbose := false
	for _, arg := range os.Args[1:] {
		if arg == "--verbose" || arg == "-v" {
			verbose = true
			break
		}
	}

	// Create client configuration directly
	cfg := client.Config{
		ApiKey: apiKey,
	}

	// Create the alert client directly
	alertClient, err := alert.NewClient(&cfg)
	if err != nil {
		log.Fatalf("❌ Failed to create OpsGenie client: %v", err)
	}

	fmt.Println("🔄 Testing connection to OpsGenie API...")

	// Test connection with a simple list request
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Testing connection by listing a single alert
	listReq := &alert.ListAlertRequest{
		Limit: 1,
	}

	if verbose {
		fmt.Println("📡 Sending request to OpsGenie API...")
		fmt.Printf("   Using API Key: %s****\n", apiKey[:4])
	}

	resp, err := alertClient.List(ctx, listReq)
	if err != nil {
		fmt.Println("❌ Connection to OpsGenie failed!")
		fmt.Printf("Error details: %v\n", err)
		fmt.Println("\nPossible solutions:")
		fmt.Println("1. Make sure you're using a FULL ACCESS API key, not a team integration key")
		fmt.Println("2. Check that your network allows connections to OpsGenie")
		fmt.Println("3. Verify that the API key has sufficient permissions (Alert:Read, Alert:Create)")
		os.Exit(1)
	}

	if verbose {
		fmt.Printf("✓ Connection successful! Response contains %d alerts\n", len(resp.Alerts))
	} else {
		fmt.Println("✅ Successfully connected to OpsGenie API!")
	}

	// Determine if an alert should be sent
	sendAlert := false
	for _, arg := range os.Args[1:] {
		if arg == "--send-alert" || arg == "-a" {
			sendAlert = true
			break
		}
	}

	// Optionally, send a test alert
	if sendAlert {
		fmt.Println("🔔 Sending test alert to OpsGenie...")

		// Create alert request directly
		alertRequest := &alert.CreateAlertRequest{
			Message:     "Test Alert from go-breaker",
			Description: "This is a test alert sent from the go-breaker circuit breaker testing tool.",
			Priority:    alert.P3,
			Source:      "go-breaker-test",
			Tags:        []string{"test", "go-breaker"},
		}

		if verbose {
			fmt.Println("📝 Alert details:")
			fmt.Printf("   Message: %s\n", alertRequest.Message)
			fmt.Printf("   Priority: %s\n", alertRequest.Priority)
			fmt.Printf("   Source: %s\n", alertRequest.Source)
		}

		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		alertResp, err := alertClient.Create(ctx, alertRequest)
		if err != nil {
			fmt.Printf("❌ Failed to send test alert: %v\n", err)
			fmt.Println("\nPossible solutions:")
			fmt.Println("1. Make sure your API key has Alert:Create permission")
			fmt.Println("2. Check that the request is properly formatted")
			os.Exit(1)
		}

		fmt.Println("✅ Successfully sent test alert to OpsGenie!")
		fmt.Printf("   Alert Request ID: %s\n", alertResp.RequestId)
		fmt.Println("   Check your OpsGenie dashboard or mobile app to verify the alert was received.")

		// Wait a bit and then send a close request
		time.Sleep(5 * time.Second)

		fmt.Println("🔄 Closing the test alert...")
		closeReq := &alert.CloseAlertRequest{
			IdentifierType:  alert.ALIAS,
			IdentifierValue: "Test Alert from go-breaker", // Using the message as alias
			Source:          "go-breaker-test",
			Note:            "Test completed successfully",
		}

		_, err = alertClient.Close(ctx, closeReq)
		if err != nil {
			fmt.Printf("⚠️ Warning: Failed to close the alert: %v\n", err)
		} else {
			fmt.Println("✅ Successfully closed the test alert!")
		}
	}

	fmt.Println("\n📝 Usage:")
	fmt.Println("  - Basic connection test: ./test_opsgenie")
	fmt.Println("  - Send test alerts:      ./test_opsgenie --send-alert")
	fmt.Println("  - Verbose output:        ./test_opsgenie --verbose")
	fmt.Println("\n👍 All tests completed successfully!")
}
