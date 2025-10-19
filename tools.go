package geminirod

import (
	"fmt"
	"maps"
	"strings"
	"time"

	computeruse "github.com/PeronGH/computer-use-lib"
	"google.golang.org/genai"
)

// builtInTools maps tool names to their handler functions
var builtInTools = map[string]func(*computeruse.Session, map[string]any) (map[string]any, error){
	"open_web_browser": handleOpenWebBrowser,
	"wait_5_seconds":   handleWait5Seconds,
	"go_back":          handleGoBack,
	"go_forward":       handleGoForward,
	"search":           handleSearch,
	"navigate":         handleNavigate,
	"click_at":         handleClickAt,
	"hover_at":         handleHoverAt,
	"type_text_at":     handleTypeTextAt,
	"key_combination":  handleKeyCombination,
	"scroll_document":  handleScrollDocument,
	"scroll_at":        handleScrollAt,
	"drag_and_drop":    handleDragAndDrop,
}

// IsBuiltInTool checks if a tool name is a built-in tool
func IsBuiltInTool(name string) bool {
	_, exists := builtInTools[name]
	return exists
}

// HandleBuiltInTool executes a built-in tool and returns a genai.Part with URL and screenshot.
// extraFields can contain additional fields like "safety_acknowledgement" to include in the response.
func HandleBuiltInTool(session *computeruse.Session, name string, args map[string]any, extraFields map[string]any) (*genai.Part, error) {
	handler, exists := builtInTools[name]
	if !exists {
		return nil, fmt.Errorf("unknown built-in tool: %s", name)
	}

	result, err := handler(session, args)
	if err != nil {
		return nil, err
	}

	// Merge extra fields (like safety_acknowledgement) into result
	maps.Copy(result, extraFields)

	// Get screenshot
	screenshot, err := session.Screenshot()
	if err != nil {
		return nil, fmt.Errorf("failed to take screenshot: %w", err)
	}

	// Create function response part with screenshot
	screenshotPart := genai.NewFunctionResponsePartFromBytes(screenshot, "image/png")

	// Create function response with URL and screenshot
	return genai.NewPartFromFunctionResponseWithParts(name, result, []*genai.FunctionResponsePart{screenshotPart}), nil
}

// Tool handlers
// All handlers return only the current URL after the operation

func getURLResponse(session *computeruse.Session) (map[string]any, error) {
	url, err := session.GetURL()
	if err != nil {
		return nil, err
	}
	return map[string]any{"url": url}, nil
}

func handleOpenWebBrowser(session *computeruse.Session, args map[string]any) (map[string]any, error) {
	// Browser should already be open with the session, so this is a no-op
	return getURLResponse(session)
}

func handleWait5Seconds(session *computeruse.Session, args map[string]any) (map[string]any, error) {
	time.Sleep(5 * time.Second)
	return getURLResponse(session)
}

func handleGoBack(session *computeruse.Session, args map[string]any) (map[string]any, error) {
	if err := session.GoBack(); err != nil {
		return nil, err
	}
	return getURLResponse(session)
}

func handleGoForward(session *computeruse.Session, args map[string]any) (map[string]any, error) {
	if err := session.GoForward(); err != nil {
		return nil, err
	}
	return getURLResponse(session)
}

func handleSearch(session *computeruse.Session, args map[string]any) (map[string]any, error) {
	if err := session.Search(); err != nil {
		return nil, err
	}
	return getURLResponse(session)
}

func handleNavigate(session *computeruse.Session, args map[string]any) (map[string]any, error) {
	url, ok := args["url"].(string)
	if !ok {
		return nil, fmt.Errorf("url argument must be a string")
	}
	if err := session.Navigate(url); err != nil {
		return nil, err
	}
	return getURLResponse(session)
}

func handleClickAt(session *computeruse.Session, args map[string]any) (map[string]any, error) {
	x, y, err := extractCoordinates(args)
	if err != nil {
		return nil, err
	}
	if err := session.ClickAt(x, y); err != nil {
		return nil, err
	}
	return getURLResponse(session)
}

func handleHoverAt(session *computeruse.Session, args map[string]any) (map[string]any, error) {
	x, y, err := extractCoordinates(args)
	if err != nil {
		return nil, err
	}
	if err := session.HoverAt(x, y); err != nil {
		return nil, err
	}
	return getURLResponse(session)
}

func handleTypeTextAt(session *computeruse.Session, args map[string]any) (map[string]any, error) {
	x, y, err := extractCoordinates(args)
	if err != nil {
		return nil, err
	}
	text, ok := args["text"].(string)
	if !ok {
		return nil, fmt.Errorf("text argument must be a string")
	}

	// Optional arguments with defaults
	pressEnter := true
	if val, exists := args["press_enter"]; exists {
		if boolVal, ok := val.(bool); ok {
			pressEnter = boolVal
		}
	}

	clearBefore := true
	if val, exists := args["clear_before_typing"]; exists {
		if boolVal, ok := val.(bool); ok {
			clearBefore = boolVal
		}
	}

	if err := session.TypeTextAt(x, y, text, clearBefore, pressEnter); err != nil {
		return nil, err
	}
	return getURLResponse(session)
}

func handleKeyCombination(session *computeruse.Session, args map[string]any) (map[string]any, error) {
	keys, ok := args["keys"].(string)
	if !ok {
		return nil, fmt.Errorf("keys argument must be a string")
	}
	// Split by "+" to convert "Control+C" to ["Control", "C"]
	// This matches the Python reference implementation and computer-use-lib's expected format
	keyParts := strings.Split(keys, "+")
	if err := session.Key(keyParts...); err != nil {
		return nil, err
	}
	return getURLResponse(session)
}

func handleScrollDocument(session *computeruse.Session, args map[string]any) (map[string]any, error) {
	direction, ok := args["direction"].(string)
	if !ok {
		return nil, fmt.Errorf("direction argument must be a string")
	}
	// Use default scroll amount (800 based on 1000x1000 grid)
	if err := session.Scroll(direction, 800); err != nil {
		return nil, err
	}
	return getURLResponse(session)
}

func handleScrollAt(session *computeruse.Session, args map[string]any) (map[string]any, error) {
	x, y, err := extractCoordinates(args)
	if err != nil {
		return nil, err
	}
	direction, ok := args["direction"].(string)
	if !ok {
		return nil, fmt.Errorf("direction argument must be a string")
	}

	magnitude := 800 // default
	if val, ok := args["magnitude"].(float64); ok {
		magnitude = int(val)
	} else if val, ok := args["magnitude"].(int); ok {
		magnitude = val
	}

	if err := session.ScrollAt(x, y, direction, magnitude); err != nil {
		return nil, err
	}
	return getURLResponse(session)
}

func handleDragAndDrop(session *computeruse.Session, args map[string]any) (map[string]any, error) {
	x, y, err := extractCoordinates(args)
	if err != nil {
		return nil, err
	}

	destX, ok := args["destination_x"].(float64)
	if !ok {
		if destXInt, ok := args["destination_x"].(int); ok {
			destX = float64(destXInt)
		} else {
			return nil, fmt.Errorf("destination_x argument must be a number")
		}
	}

	destY, ok := args["destination_y"].(float64)
	if !ok {
		if destYInt, ok := args["destination_y"].(int); ok {
			destY = float64(destYInt)
		} else {
			return nil, fmt.Errorf("destination_y argument must be a number")
		}
	}

	if err := session.ClickDrag(x, y, int(destX), int(destY)); err != nil {
		return nil, err
	}
	return getURLResponse(session)
}

// Helper functions

func extractCoordinates(args map[string]any) (int, int, error) {
	xVal, ok := args["x"].(float64)
	if !ok {
		if xInt, ok := args["x"].(int); ok {
			xVal = float64(xInt)
		} else {
			return 0, 0, fmt.Errorf("x argument must be a number")
		}
	}

	yVal, ok := args["y"].(float64)
	if !ok {
		if yInt, ok := args["y"].(int); ok {
			yVal = float64(yInt)
		} else {
			return 0, 0, fmt.Errorf("y argument must be a number")
		}
	}

	return int(xVal), int(yVal), nil
}
