package llm_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	aoption "github.com/anthropics/anthropic-sdk-go/option"
	ooption "github.com/openai/openai-go/v3/option"

	"github.com/jinho-yoo-jack/devsquad/internal/llm"
	"github.com/jinho-yoo-jack/devsquad/internal/llm/anthropic"
	"github.com/jinho-yoo-jack/devsquad/internal/llm/openai"
)

func TestOpenAIToolRoundTrip(t *testing.T) {
	var count atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Error(r.URL.Path)
		}
		var body struct {
			Model    string `json:"model"`
			Messages []struct {
				Role       string `json:"role"`
				ToolCallID string `json:"tool_call_id"`
				ToolCalls  []struct {
					ID       string `json:"id"`
					Function struct {
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"messages"`
			Tools []struct {
				Function struct {
					Name string `json:"name"`
				} `json:"function"`
			} `json:"tools"`
		}
		if e := json.NewDecoder(r.Body).Decode(&body); e != nil {
			t.Error(e)
		}
		if body.Model != "fixture" || len(body.Tools) != 1 || body.Tools[0].Function.Name != "write_file" {
			t.Errorf("bad request: %+v", body)
		}
		if count.Add(1) == 2 {
			if len(body.Messages) != 4 || body.Messages[2].ToolCalls[0].ID != "call_1" || body.Messages[3].ToolCallID != "call_1" {
				t.Errorf("tool transcript: %+v", body.Messages)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chat-1","object":"chat.completion","created":1,"model":"fixture","choices":[{"index":0,"finish_reason":"tool_calls","message":{"role":"assistant","content":"writing","tool_calls":[{"type":"function","id":"call_1","function":{"name":"write_file","arguments":"{\"path\":\"docs/a.md\"}"}}]}}],"usage":{"prompt_tokens":12,"completion_tokens":7,"prompt_tokens_details":{"cached_tokens":4}}}`))
	}))
	defer server.Close()
	adapter := openai.New("fixture", ooption.WithBaseURL(server.URL+"/v1"), ooption.WithAPIKey("test-key"))
	req := llm.ChatRequest{System: "system", Messages: []llm.Message{{Role: "user", Text: "write"}}, Tools: []llm.ToolSpec{{Name: "write_file", Schema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}},"required":["path"]}`)}}}
	turn, e := adapter.Chat(context.Background(), req)
	if e != nil {
		t.Fatal(e)
	}
	if len(turn.ToolCalls) != 1 || !json.Valid(turn.ToolCalls[0].Args) || turn.Usage.Input != 12 || turn.Usage.CacheRead != 4 {
		t.Fatal(turn)
	}
	req.Messages = append(req.Messages, llm.Message{Role: "assistant", Text: turn.Text, Calls: turn.ToolCalls}, llm.Message{Role: "tool", CallID: turn.ToolCalls[0].ID, Text: "wrote"})
	if _, e = adapter.Chat(context.Background(), req); e != nil {
		t.Fatal(e)
	}
}
func TestAnthropicCacheAndToolResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Error(r.URL.Path)
		}
		var body struct {
			System []struct {
				CacheControl struct {
					Type string `json:"type"`
				} `json:"cache_control"`
			} `json:"system"`
			Messages []struct {
				Role    string `json:"role"`
				Content []struct {
					Type      string `json:"type"`
					ToolUseID string `json:"tool_use_id"`
				} `json:"content"`
			} `json:"messages"`
		}
		if e := json.NewDecoder(r.Body).Decode(&body); e != nil {
			t.Error(e)
		}
		if len(body.System) != 1 || body.System[0].CacheControl.Type != "ephemeral" {
			t.Error("missing cache control")
		}
		if len(body.Messages) != 3 || len(body.Messages[2].Content) != 2 || body.Messages[2].Content[1].ToolUseID != "call_2" {
			t.Errorf("tool results not grouped: %+v", body.Messages)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"msg_1","type":"message","role":"assistant","model":"fixture","stop_reason":"end_turn","content":[{"type":"text","text":"done"}],"usage":{"input_tokens":8,"output_tokens":2,"cache_read_input_tokens":40,"cache_creation_input_tokens":12}}`))
	}))
	defer server.Close()
	adapter := anthropic.New("fixture", aoption.WithBaseURL(server.URL), aoption.WithAPIKey("test-key"))
	out, e := adapter.Chat(context.Background(), llm.ChatRequest{System: "static context", Messages: []llm.Message{{Role: "user", Text: "read"}, {Role: "assistant", Calls: []llm.ToolCall{{ID: "call_1", Name: "read_file", Args: json.RawMessage(`{"path":"a"}`)}, {ID: "call_2", Name: "read_file", Args: json.RawMessage(`{"path":"b"}`)}}}, {Role: "tool", CallID: "call_1", Text: "a"}, {Role: "tool", CallID: "call_2", Text: "[denied] b"}}})
	if e != nil {
		t.Fatal(e)
	}
	if out.Text != "done" || out.Usage.CacheRead != 40 || out.Usage.CacheWrite != 12 {
		t.Fatal(out)
	}
}
func TestSDKRetriesAndCancellation(t *testing.T) {
	var count atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.Header().Set("retry-after-ms", "1")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(429)
		_, _ = w.Write([]byte(`{"error":{"message":"busy","type":"rate_limit_error"}}`))
	}))
	defer server.Close()
	adapter := openai.New("fixture", ooption.WithBaseURL(server.URL), ooption.WithAPIKey("test-key"))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, e := adapter.Chat(ctx, llm.ChatRequest{}); e == nil {
		t.Fatal("429 succeeded")
	}
	if count.Load() != 4 {
		t.Fatalf("expected initial request + 3 retries, got %d", count.Load())
	}
	cancel()
	before := count.Load()
	if _, e := adapter.Chat(ctx, llm.ChatRequest{}); e == nil {
		t.Fatal("cancelled request succeeded")
	}
	if count.Load() != before {
		t.Fatal("cancelled request reached server")
	}
}
