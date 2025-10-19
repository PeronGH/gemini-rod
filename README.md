# gemini-rod

Go library and demo for integrating Google's Gemini Computer Use with browser automation.

## Quick Start

### As a Library

```bash
go get github.com/PeronGH/gemini-rod
```

```go
package main

import (
    "context"
    "os"
    computeruse "github.com/PeronGH/computer-use-lib"
    geminirod "github.com/PeronGH/gemini-rod"
    "google.golang.org/genai"
)

func main() {
    ctx := context.Background()

    session, _ := computeruse.NewSession(ctx, computeruse.SessionConfig{
        NormalizeCoordinates: true,
    })
    defer session.Close()

    client, _ := genai.NewClient(ctx, &genai.ClientConfig{
        APIKey: os.Getenv("GEMINI_API_KEY"),
    })

    eventChan := geminirod.StartLoop(ctx, geminirod.StartLoopConfig{
        GenaiClient:        client,
        ComputerUseSession: session,
        Prompt:             "Search for Go tutorials on Google",
    })

    for event := range eventChan {
        // Handle ProgressEvent, SafetyConfirmationEvent, ErrorEvent
    }
}
```

### Running the Demo

```bash
export GEMINI_API_KEY="your-api-key"
cd examples
go run ./basic -help
```
