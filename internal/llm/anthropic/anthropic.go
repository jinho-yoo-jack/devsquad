package anthropic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/jinho-yoo-jack/devsquad/internal/llm"
)

type Adapter struct {
	Client sdk.Client
	Model  string
}

func New(model string, opts ...option.RequestOption) *Adapter {
	opts = append([]option.RequestOption{option.WithMaxRetries(3)}, opts...)
	return &Adapter{Client: sdk.NewClient(opts...), Model: model}
}
func (a *Adapter) Chat(ctx context.Context, r llm.ChatRequest) (llm.ChatResponse, error) {
	maxTokens := r.MaxTokens
	if maxTokens == 0 {
		maxTokens = 8192
	}
	p := sdk.MessageNewParams{Model: sdk.Model(a.Model), MaxTokens: int64(maxTokens), System: []sdk.TextBlockParam{{Text: r.System, CacheControl: sdk.NewCacheControlEphemeralParam()}}}
	for _, m := range r.Messages {
		if m.Role == "tool" {
			block := sdk.NewToolResultBlock(m.CallID, m.Text, strings.HasPrefix(m.Text, "[denied]") || strings.HasPrefix(m.Text, "[bad args]") || strings.HasPrefix(m.Text, "[not found]"))
			if len(p.Messages) > 0 && p.Messages[len(p.Messages)-1].Role == "user" {
				p.Messages[len(p.Messages)-1].Content = append(p.Messages[len(p.Messages)-1].Content, block)
			} else {
				p.Messages = append(p.Messages, sdk.NewUserMessage(block))
			}
			continue
		}
		blocks := []sdk.ContentBlockParamUnion{}
		if m.Text != "" {
			blocks = append(blocks, sdk.NewTextBlock(m.Text))
		}
		for _, t := range m.Calls {
			if !json.Valid(t.Args) {
				return llm.ChatResponse{}, fmt.Errorf("invalid tool arguments")
			}
			blocks = append(blocks, sdk.NewToolUseBlock(t.ID, t.Args, t.Name))
		}
		if m.Role == "assistant" {
			p.Messages = append(p.Messages, sdk.NewAssistantMessage(blocks...))
		} else if m.Role == "user" {
			p.Messages = append(p.Messages, sdk.NewUserMessage(blocks...))
		} else {
			return llm.ChatResponse{}, fmt.Errorf("unsupported message role: %s", m.Role)
		}
	}
	for _, t := range r.Tools {
		var schema sdk.ToolInputSchemaParam
		if err := json.Unmarshal(t.Schema, &schema); err != nil {
			return llm.ChatResponse{}, err
		}
		p.Tools = append(p.Tools, sdk.ToolUnionParam{OfTool: &sdk.ToolParam{Name: t.Name, Description: sdk.String(t.Description), InputSchema: schema}})
	}
	res, err := a.Client.Messages.New(ctx, p)
	if err != nil {
		if ctx.Err() != nil {
			return llm.ChatResponse{}, ctx.Err()
		}
		var api *sdk.Error
		if errors.As(err, &api) {
			return llm.ChatResponse{}, fmt.Errorf("anthropic request failed: HTTP %d", api.StatusCode)
		}
		return llm.ChatResponse{}, fmt.Errorf("anthropic request failed")
	}
	out := llm.ChatResponse{Model: "anthropic/" + a.Model, Usage: llm.Usage{Input: res.Usage.InputTokens, Output: res.Usage.OutputTokens, CacheRead: res.Usage.CacheReadInputTokens, CacheWrite: res.Usage.CacheCreationInputTokens}}
	for _, b := range res.Content {
		switch b.Type {
		case "text":
			out.Text += b.Text
		case "tool_use":
			out.ToolCalls = append(out.ToolCalls, llm.ToolCall{ID: b.ID, Name: b.Name, Args: b.Input})
		}
	}
	return out, nil
}
