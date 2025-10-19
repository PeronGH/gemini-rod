package geminirod

import (
	"context"
	"fmt"

	computeruse "github.com/PeronGH/computer-use-lib"
	"google.golang.org/genai"
)

type StartLoopConfig struct {
	GenaiClient        *genai.Client
	ComputerUseSession *computeruse.Session
	ExtraTools         []*genai.Tool
	Prompt             string
	Model              string
}

func StartLoop(ctx context.Context, config *StartLoopConfig) <-chan Event {
	eventChan := make(chan Event)

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
			var text string
			for _, part := range resp.Candidates[0].Content.Parts {
				if part.Text != "" {
					text += part.Text
				}
			}

			functionCalls := resp.FunctionCalls()

			// Create function call events
			var callEvents []*FunctionCall
			for _, fc := range functionCalls {
				callEvents = append(callEvents, &FunctionCall{
					FunctionName: fc.Name,
					Args:         fc.Args,
					needsAction:  true, // TODO: determine based on whether it's an extra tool or computer use
					respondFunc: func(response any) error {
						// TODO: add function response to history
						return nil
					},
				})
			}

			// Send progress event
			eventChan <- ProgressEvent{
				Text:          text,
				FunctionCalls: callEvents,
			}

			// If there is no function call, end the loop
			if len(functionCalls) == 0 {
				break
			}

			// TODO: wait for responses from subscriber before continuing
			// TODO: a mechanism to handle tool calls from extra tools
			// TODO: handle computer use tool calls
		}
	}()

	return eventChan
}
