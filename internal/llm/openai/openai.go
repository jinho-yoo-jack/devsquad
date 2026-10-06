package openai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	sdk "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"

	"github.com/jinho-yoo-jack/devsquad/internal/llm"
)

type Adapter struct {
	Client          sdk.Client
	Model, Provider string
}

func New(model string, opts ...option.RequestOption) *Adapter {
	opts = append([]option.RequestOption{option.WithMaxRetries(3)}, opts...)
	return &Adapter{Client: sdk.NewClient(opts...), Model: model, Provider: "openai"}
}
func (a *Adapter) Chat(ctx context.Context, r llm.ChatRequest) (llm.ChatResponse, error) {
	maxTokens := r.MaxTokens
	if maxTokens == 0 {
		maxTokens = 8192
	}
	p := sdk.ChatCompletionNewParams{Model: a.Model, MaxCompletionTokens: sdk.Int(int64(maxTokens)), Messages: []sdk.ChatCompletionMessageParamUnion{sdk.SystemMessage(r.System)}}
	for _, m := range r.Messages {
		switch m.Role {
		case "user":
			p.Messages = append(p.Messages, sdk.UserMessage(m.Text))
		case "tool":
			p.Messages = append(p.Messages, sdk.ToolMessage(m.Text, m.CallID))
		case "assistant":
			msg := sdk.AssistantMessage(m.Text)
			for _, t := range m.Calls {
				msg.OfAssistant.ToolCalls = append(msg.OfAssistant.ToolCalls, sdk.ChatCompletionMessageToolCallUnionParam{OfFunction: &sdk.ChatCompletionMessageFunctionToolCallParam{ID: t.ID, Function: sdk.ChatCompletionMessageFunctionToolCallFunctionParam{Name: t.Name, Arguments: string(t.Args)}}})
			}
			p.Messages = append(p.Messages, msg)
		default:
			return llm.ChatResponse{}, fmt.Errorf("unsupported message role: %s", m.Role)
		}
	}
	for _, t := range r.Tools {
		var schema shared.FunctionParameters
		if err := json.Unmarshal(t.Schema, &schema); err != nil {
			return llm.ChatResponse{}, err
		}
		p.Tools = append(p.Tools, sdk.ChatCompletionFunctionTool(shared.FunctionDefinitionParam{Name: t.Name, Description: sdk.String(t.Description), Parameters: schema}))
	}
	res, err := a.Client.Chat.Completions.New(ctx, p)
	if err != nil {
		if ctx.Err() != nil {
			return llm.ChatResponse{}, ctx.Err()
		}
		var api *sdk.Error
		if errors.As(err, &api) {
			return llm.ChatResponse{}, fmt.Errorf("%s request failed: HTTP %d", a.Provider, api.StatusCode)
		}
		return llm.ChatResponse{}, fmt.Errorf("%s request failed", a.Provider)
	}
	if len(res.Choices) == 0 {
		return llm.ChatResponse{}, fmt.Errorf("%s returned no choices", a.Provider)
	}
	out := llm.ChatResponse{Model: a.Provider + "/" + a.Model, Text: res.Choices[0].Message.Content, Usage: llm.Usage{Input: res.Usage.PromptTokens, Output: res.Usage.CompletionTokens, CacheRead: res.Usage.PromptTokensDetails.CachedTokens}}
	for _, t := range res.Choices[0].Message.ToolCalls {
		if t.Type != "function" {
			return out, fmt.Errorf("unsupported tool call type: %s", t.Type)
		}
		var args map[string]json.RawMessage
		if err := json.Unmarshal([]byte(t.Function.Arguments), &args); err != nil || args == nil {
			return out, fmt.Errorf("invalid tool arguments for %s", t.Function.Name)
		}
		out.ToolCalls = append(out.ToolCalls, llm.ToolCall{ID: t.ID, Name: t.Function.Name, Args: json.RawMessage(t.Function.Arguments)})
	}
	return out, nil
}
