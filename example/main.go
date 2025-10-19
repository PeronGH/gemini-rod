package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	computeruse "github.com/PeronGH/computer-use-lib"
	geminirod "github.com/PeronGH/gemini-rod"
	"google.golang.org/genai"
)

const screenWidth = 1440
const screenHeight = 900

func main() {
	// Parse command-line arguments
	query := flag.String("query", "", "The query for the browser agent to execute.")
	initialURL := flag.String("initial-url", "https://www.google.com", "The initial URL loaded for the computer.")
	model := flag.String("model", "gemini-2.5-computer-use-preview-10-2025", "Set which main model to use.")
	flag.Parse()

	if *query == "" {
		log.Fatal("Error: --query flag is required")
	}

	// Create context
	ctx := context.Background()

	// Initialize computer use session
	session, err := computeruse.NewSession(ctx, computeruse.SessionConfig{
		ScreenWidth:          screenWidth,
		ScreenHeight:         screenHeight,
		InitialURL:           *initialURL,
		NormalizeCoordinates: true,
	})
	if err != nil {
		log.Fatalf("Failed to create computer use session: %v", err)
	}
	defer func() {
		if err := session.Close(); err != nil {
			log.Printf("Failed to close session: %v", err)
		}
	}()

	// Initialize Genai client
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: os.Getenv("GEMINI_API_KEY"),
	})
	if err != nil {
		log.Fatalf("Failed to create genai client: %v", err)
	}

	// Start the agent loop
	eventChan := geminirod.StartLoop(ctx, &geminirod.StartLoopConfig{
		GenaiClient:                   client,
		ComputerUseSession:            session,
		ExtraTools:                    nil,
		Prompt:                        *query,
		Model:                         *model,
		MaxRecentTurnsWithScreenshots: 3,
	})

	// Process events
	for event := range eventChan {
		switch e := event.(type) {
		case geminirod.ProgressEvent:
			// Print reasoning/text if present
			if e.Text != "" {
				fmt.Printf("\nGemini Computer Use Reasoning:\n%s\n", e.Text)
			}

			// Handle function calls
			if len(e.FunctionCalls) > 0 {
				fmt.Println("\nFunction Call(s):")
				for _, fc := range e.FunctionCalls {
					fmt.Printf("Name: %s\n", fc.FunctionName)
					if len(fc.Args) > 0 {
						fmt.Println("Args:")
						for key, value := range fc.Args {
							fmt.Printf("  %s: %v\n", key, value)
						}
					}
				}
				fmt.Println()
			}

		case geminirod.ErrorEvent:
			log.Fatalf("Error: %v", e.Err)
		}
	}

	fmt.Println("Agent Loop Complete")
}
