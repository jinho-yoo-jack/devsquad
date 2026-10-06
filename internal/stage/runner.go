package stage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jinho-yoo-jack/devsquad/internal/agentdef"
	"github.com/jinho-yoo-jack/devsquad/internal/event"
	"github.com/jinho-yoo-jack/devsquad/internal/llm"
	"github.com/jinho-yoo-jack/devsquad/internal/pipeline"
	"github.com/jinho-yoo-jack/devsquad/internal/tools"
)

type Input struct {
	TaskID, Command, Mode, Plan, Feedback, Inputs string
	Stage                                         pipeline.StageSpec
	Definition                                    agentdef.Definition
	RetryNo                                       int
	Emit                                          func(context.Context, string, any) error
}
type Result struct {
	Content      string
	Ref          *string
	WrittenPaths []string
	Iterations   int
	HitLimit     bool
}
type Runner interface {
	Run(context.Context, Input) (Result, error)
}
type AgentRunner struct {
	Registry    llm.Registry
	Budget      llm.BudgetGuard
	TestTimeout time.Duration
}

func Model(d agentdef.Definition, s pipeline.StageSpec) string {
	if s.Model != "" {
		return s.Model
	}
	if d.Agents[s.Agent].Model != "" {
		return d.Agents[s.Agent].Model
	}
	return d.Pipeline.Policy.DefaultModel
}
func (a *AgentRunner) Run(ctx context.Context, in Input) (Result, error) {
	out := Result{WrittenPaths: []string{}}
	root, err := os.OpenRoot(in.Definition.Workspace)
	if err != nil {
		return out, err
	}
	defer root.Close()
	agent := in.Definition.Agents[in.Stage.Agent]
	sandbox := tools.Sandbox{Root: root, Agent: agent, Timeout: a.TestTimeout, Allowed: in.Stage.Tools}
	model, err := a.Registry.Get(Model(in.Definition, in.Stage))
	if err != nil {
		return out, err
	}
	system, user := Prompts(in, agent)
	messages := []llm.Message{{Role: "user", Text: user}}
	var specs []llm.ToolSpec
	if in.Mode == "execute" {
		specs = sandbox.Tools()
	}
	limit := in.Definition.Pipeline.Policy.MaxIterations
	if in.Mode == "plan" {
		limit = 1
	}
	budget := a.Budget
	if budget == nil {
		budget = llm.NoopBudget{}
	}
	modelName := Model(in.Definition, in.Stage)
	if a.Registry.ForceFake || modelName == "" || modelName == "fake" {
		modelName = "fake/echo"
	}
	for i := 0; i < limit; i++ {
		if err = budget.Check(ctx, in.TaskID); err != nil {
			return out, err
		}
		if err = in.Emit(ctx, "agent.thinking", map[string]any{"summary": in.Mode, "iteration": i + 1, "model": modelName}); err != nil {
			return out, err
		}
		turn, e := model.Chat(ctx, llm.ChatRequest{System: system, Messages: messages, Tools: specs, MaxTokens: 8192})
		if e != nil {
			return out, e
		}
		out.Iterations = i + 1
		if e = in.Emit(ctx, "usage", event.UsagePayload{Model: turn.Model, Usage: turn.Usage}); e != nil {
			return out, e
		}
		messages = append(messages, llm.Message{Role: "assistant", Text: turn.Text, Calls: turn.ToolCalls})
		if len(turn.ToolCalls) == 0 {
			out.Content = turn.Text
			break
		}
		if in.Mode == "plan" {
			return out, fmt.Errorf("plan phase must not call tools")
		}
		for _, tc := range turn.ToolCalls {
			if e = ctx.Err(); e != nil {
				return out, e
			}
			if e = in.Emit(ctx, "agent.tool_call", map[string]any{"call_id": tc.ID, "tool": tc.Name, "args_summary": Truncate(string(tc.Args), 200)}); e != nil {
				return out, e
			}
			start := time.Now()
			observation, callErr := sandbox.Call(ctx, tc.Name, tc.Args)
			if callErr != nil {
				prefix := "[denied] "
				var args tools.ToolArgs
				if !json.Valid(tc.Args) || json.Unmarshal(tc.Args, &args) != nil {
					prefix = "[bad args] "
				} else if os.IsNotExist(callErr) {
					prefix = "[not found] "
				}
				observation = prefix + callErr.Error()
			} else if tc.Name == "write_file" {
				var args tools.ToolArgs
				if e = json.Unmarshal(tc.Args, &args); e != nil {
					return out, e
				}
				out.WrittenPaths = append(out.WrittenPaths, args.Path)
			}
			if e = in.Emit(ctx, "agent.tool_result", map[string]any{"call_id": tc.ID, "tool": tc.Name, "ok": callErr == nil, "summary": Truncate(observation, 200), "duration_ms": time.Since(start).Milliseconds()}); e != nil {
				return out, e
			}
			messages = append(messages, llm.Message{Role: "tool", Text: observation, CallID: tc.ID})
		}
		if i == limit-1 {
			out.HitLimit = true
			out.Content = "[iteration limit reached — submitting current deliverable]"
			if e = in.Emit(ctx, "agent.message", map[string]any{"text": out.Content, "hit_limit": true}); e != nil {
				return out, e
			}
		}
	}
	if in.Mode == "execute" && len(out.WrittenPaths) > 0 {
		primary := out.WrittenPaths[len(out.WrittenPaths)-1]
		out.Ref = &primary
		out.Content, err = sandbox.Read(primary)
		if err != nil {
			return out, err
		}
		if !strings.HasPrefix(out.Content, "> Task:") {
			out.Content = fmt.Sprintf("> Task: %s · 작성: %s · 버전: v%d\n\n", in.TaskID, agent.Name, in.RetryNo+1) + out.Content
			if err = sandbox.Write(primary, out.Content); err != nil {
				return out, err
			}
		}
	}
	return out, nil
}
func Truncate(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}
func Prompts(in Input, a agentdef.AgentSpec) (string, string) {
	mission := "Write only a short plan following conventions. Do not use tools or write files."
	if in.Mode == "execute" {
		mission = "Execute the approved plan, write deliverables, and check completion criteria. Include rejection feedback in the revision."
	}
	system := fmt.Sprintf("<persona>\n%s\n</persona>\n<conventions>\n%s\n</conventions>\n<project>\n%s\n</project>\n<mission>\n%s\n</mission>\n<knowledge>\n%s\n</knowledge>\n<tools>\nprofile=%s; write_paths=%q\n</tools>", a.Persona, a.Conventions, in.Definition.ProjectSpec, mission, a.Knowledge, a.ToolProfile, a.WritePaths)
	user := fmt.Sprintf("[[MODE:%s]]\n[[COMMAND]]\n%s\n[[/COMMAND]]\n<inputs>\n%s\n</inputs>", in.Mode, in.Command, in.Inputs)
	if in.Plan != "" {
		user += "\n<approved_plan>\n" + in.Plan + "\n</approved_plan>"
	}
	if in.Feedback != "" {
		user += "\n[[FEEDBACK]]\n" + in.Feedback + "\n[[/FEEDBACK]]"
	}
	return system, user
}
