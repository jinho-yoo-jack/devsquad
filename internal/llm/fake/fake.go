package fake

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/jinho-yoo-jack/devsquad/internal/llm"
)

type LLM struct{}

func marker(text, name string) string {
	a := strings.Index(text, "[["+name+"]]")
	if a < 0 {
		return ""
	}
	text = text[a+len(name)+4:]
	b := strings.Index(text, "[[/"+name+"]]")
	if b < 0 {
		return ""
	}
	return strings.TrimSpace(text[:b])
}
func (LLM) Chat(ctx context.Context, r llm.ChatRequest) (llm.ChatResponse, error) {
	if err := ctx.Err(); err != nil {
		return llm.ChatResponse{}, err
	}
	user := ""
	n := len(r.System)
	var last *llm.Message
	for i, m := range r.Messages {
		n += len(m.Text)
		if m.Role == "user" && user == "" {
			user = m.Text
		}
		if m.Role == "tool" {
			last = &r.Messages[i]
		}
	}
	words := regexp.MustCompile(`[A-Za-z0-9가-힣]+`).FindAllString(marker(user, "COMMAND"), 4)
	slug := strings.ToLower(strings.Join(words, "-"))
	if slug == "" {
		slug = "task"
	}
	feedback := marker(user, "FEEDBACK")
	tail := ""
	if feedback != "" {
		tail = "\n## 반려 반영 내역\n- " + feedback + "\n"
	}
	body := "# " + slug + " 요구사항 정의서 (fake)\n" + tail + "\n## 1. 배경과 목표\n(fake)\n\n## 4. 유스케이스\n### UC-001: (fake)\n- 사전조건:\n- 주 흐름:\n  1. …\n- 대안·예외 흐름:\n  - E1 …\n- 사후조건:\n\n```yaml\n# devsquad:summary\nuse_cases: [UC-001]\nscreens: []\nentities: []\n```\n"
	out := llm.ChatResponse{Model: "fake/echo", Usage: llm.Usage{Input: int64(n / 4)}}
	if strings.Contains(user, "[[MODE:plan]]") {
		out.Text = fmt.Sprintf("## 계획 — %s\n1. 요청 해석: (fake) 요청을 요구사항 정의서로 구조화한다\n2. 작성할 문서: docs/spec/%s.md\n3. 포함할 유스케이스 (예상): UC-001, UC-002\n4. 확인이 필요한 질문: 없음\n5. 범위 밖으로 둘 것: 없음\n", slug, slug)
		if feedback != "" {
			out.Text += "6. 반려 반영: " + feedback
		}
	} else if len(r.Tools) == 0 {
		out.Text = body
	} else {
		writable := false
		for _, t := range r.Tools {
			writable = writable || t.Name == "write_file"
		}
		if writable && (last == nil || strings.HasPrefix(last.Text, "[denied]")) {
			target := regexp.MustCompile(`docs/[\w\-/]+\.md`).FindString(user)
			if target == "" {
				target = "docs/spec/" + slug + ".md"
			}
			id := "call_1"
			if last != nil {
				id = "call_2"
				target = "docs/fallback.md"
				m := regexp.MustCompile(`write_paths=\["([^"*]+)`).FindStringSubmatch(last.Text)
				if len(m) > 1 {
					target = strings.TrimRight(m[1], "/") + "/fallback.md"
				}
			}
			args, _ := json.Marshal(struct {
				Path    string `json:"path"`
				Content string `json:"content"`
			}{target, body})
			out.ToolCalls = []llm.ToolCall{{ID: id, Name: "write_file", Args: args}}
			out.Text = "결과물을 파일로 작성합니다."
		} else {
			out.Text = "완료 기준 자가 점검 완료. 결과물을 제출합니다."
		}
	}
	out.Usage.Output = int64(len(out.Text) / 4)
	return out, nil
}
