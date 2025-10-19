package geminirod

import (
	"context"
	"fmt"

	computeruse "github.com/PeronGH/computer-use-lib"
	"google.golang.org/genai"
)

type StartLoopConfig struct {
	GenaiClient                   *genai.Client
	ComputerUseSession            *computeruse.Session
	ExtraTools                    []*genai.Tool
	Prompt                        string
	Model                         string // Default: "gemini-2.5-computer-use-preview-10-2025"
	MaxRecentTurnsWithScreenshots int    // Maximum number of recent turns with screenshots to keep in history. Default: 3, -1 = unlimited
}

func StartLoop(ctx context.Context, config StartLoopConfig) <-chan Event {
	eventChan := make(chan Event)

	// Apply defaults
	if config.Model == "" {
		config.Model = "gemini-2.5-computer-use-preview-10-2025"
	}
	if config.MaxRecentTurnsWithScreenshots == 0 {
		config.MaxRecentTurnsWithScreenshots = 3
	}

	go func() {
		defer close(eventChan)

		history := []*genai.Content{
			{
				Role: genai.RoleUser,
				Parts: []*genai.Part{
					{Text: config.Prompt},
				},
			},
		}

		generateContentConfig := &genai.GenerateContentConfig{
			Temperature: genai.Ptr[float32](0.2),
			Tools: append(config.ExtraTools, &genai.Tool{
				ComputerUse: &genai.ComputerUse{
					Environment: genai.EnvironmentBrowser,
				},
			}),
			ThinkingConfig: &genai.ThinkingConfig{
				IncludeThoughts: true,
			},
		}

		for {
			// Check context cancellation
			select {
			case <-ctx.Done():
				eventChan <- ErrorEvent{Err: ctx.Err()}
				return
			default:
			}

			// Send the request
			resp, err := config.GenaiClient.Models.GenerateContent(ctx, config.Model, history, generateContentConfig)
			if err != nil {
				eventChan <- ErrorEvent{Err: fmt.Errorf("error during generating content: %w", err)}
				return
			}

			// Update history with newly generated message
			history = append(history, resp.Candidates[0].Content)

			// Extract text and function calls from response
			text := extractText(resp.Candidates[0].Content)
			functionCalls := resp.FunctionCalls()

			// If there is no function call, end the loop
			if len(functionCalls) == 0 {
				eventChan <- ProgressEvent{
					Text:          text,
					FunctionCalls: nil,
				}
				break
			}

			// Create function call events and prepare for responses
			callEvents, pendingResponses := createFunctionCallEvents(functionCalls)

			// Send progress event
			eventChan <- ProgressEvent{
				Text:          text,
				FunctionCalls: callEvents,
			}

			// Execute function calls and collect responses
			responseParts, err := executeFunctionCalls(ctx, config.ComputerUseSession, functionCalls, pendingResponses)
			if err != nil {
				eventChan <- ErrorEvent{Err: err}
				return
			}

			// Add function responses to history
			history = append(history, &genai.Content{
				Role:  genai.RoleUser,
				Parts: responseParts,
			})

			// Prune old screenshots to keep context size manageable (-1 means unlimited)
			if config.MaxRecentTurnsWithScreenshots > 0 {
				pruneOldScreenshots(history, config.MaxRecentTurnsWithScreenshots)
			}
		}
	}()

	return eventChan
}

// extractText extracts all text parts from a content
func extractText(content *genai.Content) string {
	var text string
	for _, part := range content.Parts {
		if part.Text != "" {
			text += part.Text
		}
	}
	return text
}

// pendingResponse holds the channels for communicating with custom tool handlers
type pendingResponse struct {
	funcCall   *genai.FunctionCall
	respChan   chan map[string]any
	rejectChan chan error
}

// createFunctionCallEvents creates FunctionCall events and prepares response channels
func createFunctionCallEvents(functionCalls []*genai.FunctionCall) ([]*FunctionCall, []*pendingResponse) {
	var callEvents []*FunctionCall
	var pendingResponses []*pendingResponse

	for _, fc := range functionCalls {
		funcCall := fc // capture for closure
		isBuiltIn := IsBuiltInTool(funcCall.Name)

		if isBuiltIn {
			// Built-in tools are handled automatically
			callEvents = append(callEvents, &FunctionCall{
				FunctionName: funcCall.Name,
				Args:         funcCall.Args,
				needsAction:  false,
				respondFunc:  nil,
			})
		} else {
			// Custom tools need subscriber to handle
			respChan := make(chan map[string]any)
			rejectChan := make(chan error)

			pending := &pendingResponse{
				funcCall:   funcCall,
				respChan:   respChan,
				rejectChan: rejectChan,
			}
			pendingResponses = append(pendingResponses, pending)

			callEvents = append(callEvents, &FunctionCall{
				FunctionName: funcCall.Name,
				Args:         funcCall.Args,
				needsAction:  true,
				respondFunc: func(response map[string]any) {
					respChan <- response
				},
				rejectFunc: func(err error) {
					rejectChan <- err
				},
			})
		}
	}

	return callEvents, pendingResponses
}

// executeFunctionCalls executes all function calls (built-in and custom) and returns response parts.
// It maintains the order of function calls to match the Python reference implementation.
func executeFunctionCalls(
	ctx context.Context,
	session *computeruse.Session,
	functionCalls []*genai.FunctionCall,
	pendingResponses []*pendingResponse,
) ([]*genai.Part, error) {
	var responseParts []*genai.Part
	pendingIdx := 0

	// Process function calls in order (built-in and custom interleaved)
	for _, fc := range functionCalls {
		if IsBuiltInTool(fc.Name) {
			// Handle built-in tool immediately
			part, err := HandleBuiltInTool(session, fc.Name, fc.Args)
			if err != nil {
				return nil, fmt.Errorf("error handling built-in tool %s: %w", fc.Name, err)
			}
			responseParts = append(responseParts, part)
		} else {
			// Wait for custom tool response from subscriber
			pending := pendingResponses[pendingIdx]
			pendingIdx++

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case err := <-pending.rejectChan:
				return nil, fmt.Errorf("function call %s rejected: %w", pending.funcCall.Name, err)
			case response := <-pending.respChan:
				// Create function response part
				part := genai.NewPartFromFunctionResponse(pending.funcCall.Name, response)
				responseParts = append(responseParts, part)
			}
		}
	}

	return responseParts, nil
}

// pruneOldScreenshots removes screenshot images from old turns to keep context size manageable.
// It keeps only the most recent maxTurns turns that contain screenshots.
func pruneOldScreenshots(history []*genai.Content, maxTurns int) {
	turnsWithScreenshotsFound := 0

	// Iterate through history in reverse to find turns with screenshots
	for i := len(history) - 1; i >= 0; i-- {
		content := history[i]
		if content.Role != genai.RoleUser || content.Parts == nil {
			continue
		}

		// Check if this content has screenshots from built-in computer use functions
		hasScreenshot := false
		for _, part := range content.Parts {
			if part.FunctionResponse != nil &&
				part.FunctionResponse.Parts != nil &&
				IsBuiltInTool(part.FunctionResponse.Name) {
				hasScreenshot = true
				break
			}
		}

		if hasScreenshot {
			turnsWithScreenshotsFound++
			// Remove screenshot images if we exceed the limit
			if turnsWithScreenshotsFound > maxTurns {
				for _, part := range content.Parts {
					if part.FunctionResponse != nil &&
						part.FunctionResponse.Parts != nil &&
						IsBuiltInTool(part.FunctionResponse.Name) {
						// Remove the screenshot parts but keep the function response
						part.FunctionResponse.Parts = nil
					}
				}
			}
		}
	}
}
