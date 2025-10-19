package geminirod

// Event represents different events that can occur during the StartLoop execution.
// This interface uses a sealed/sum-type pattern similar to Rust enums.
type Event interface {
	isEvent()
}

// ProgressEvent represents model progress including text output and function calls
type ProgressEvent struct {
	Text          string
	FunctionCalls []*FunctionCall
}

func (ProgressEvent) isEvent() {}

// ErrorEvent represents an error that occurred
type ErrorEvent struct {
	Err error
}

func (ErrorEvent) isEvent() {}

// FunctionCall represents a function call that may or may not require action from the subscriber
type FunctionCall struct {
	FunctionName string
	Args         map[string]any
	needsAction  bool
	respondFunc  func(response any) error
}

// NeedsAction returns true if this function call requires action from the subscriber
func (fc *FunctionCall) NeedsAction() bool {
	return fc.needsAction
}

// Respond sends a response back for this function call
func (fc *FunctionCall) Respond(response any) error {
	if fc.respondFunc == nil {
		return nil
	}
	return fc.respondFunc(response)
}
