package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type ToolSpec struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Schema      json.RawMessage `json:"input_schema"`
}
type ToolCall struct {
	ID, Name string
	Args     json.RawMessage
}
type Message struct {
	Role, Text, CallID string
	Calls              []ToolCall
}
type Usage struct {
	Input      int64 `json:"input_tokens"`
	Output     int64 `json:"output_tokens"`
	CacheRead  int64 `json:"cache_read_tokens"`
	CacheWrite int64 `json:"cache_write_tokens"`
}
type ChatRequest struct {
	System    string
	Messages  []Message
	Tools     []ToolSpec
	MaxTokens int
}
type ChatResponse struct {
	Text      string
	ToolCalls []ToolCall
	Model     string
	Usage     Usage
}
type LLM interface {
	Chat(context.Context, ChatRequest) (ChatResponse, error)
}
type Factory func(string) LLM
type Registry struct {
	Providers map[string]Factory
	Fake      LLM
	ForceFake bool
}

func (r Registry) Validate(model string) error {
	if model == "" || model == "fake" || model == "fake/echo" {
		return nil
	}
	provider, name, ok := strings.Cut(model, "/")
	if !ok || strings.TrimSpace(name) == "" || r.Providers[provider] == nil {
		return fmt.Errorf("model must be a supported 'provider/model': %q", model)
	}
	return nil
}
func (r Registry) Get(model string) (LLM, error) {
	if err := r.Validate(model); err != nil {
		return nil, err
	}
	if r.ForceFake || model == "" || model == "fake" || model == "fake/echo" {
		return r.Fake, nil
	}
	provider, name, _ := strings.Cut(model, "/")
	return r.Providers[provider](name), nil
}

// ErrBudgetExceeded stops a stage without failing its Task; the Task is paused.
var ErrBudgetExceeded = errors.New("token budget exceeded")

type BudgetGuard interface {
	Check(context.Context, string) error
}
type NoopBudget struct{}

func (NoopBudget) Check(context.Context, string) error { return nil }
