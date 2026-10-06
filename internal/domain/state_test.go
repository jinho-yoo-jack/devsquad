package domain

import "testing"

func TestTaskTransitions(t *testing.T) {
	allowed := map[string]map[string]string{"queued": {"start": "running", "cancel": "cancelled"}, "running": {"pause": "paused", "cancel": "cancelled"}, "waiting_approval": {"pause": "paused", "cancel": "cancelled"}, "paused": {"resume": "running", "cancel": "cancelled"}, "blocked": {"resume": "running", "cancel": "cancelled"}}
	for _, state := range []string{"queued", "running", "waiting_approval", "paused", "blocked", "completed", "failed", "cancelled"} {
		for _, action := range []string{"start", "pause", "resume", "cancel"} {
			got, e := TaskTransition(state, action)
			want, ok := allowed[state][action]
			if ok && (e != nil || got != want) {
				t.Fatalf("%s %s: %s %v", state, action, got, e)
			}
			if !ok && e == nil {
				t.Fatalf("allowed %s %s", state, action)
			}
		}
	}
}
func TestApprovalTransitions(t *testing.T) {
	for _, kind := range []string{"plan", "deliverable"} {
		for _, decision := range []string{"approve", "edit", "reject"} {
			for retry := 0; retry < 3; retry++ {
				s := StageEntity{Status: kind + "_review", Plan: Ptr("old plan"), DeliverableSummary: Ptr("old summary")}
				if kind == "plan" {
					s.PlanRetryNo = retry
				} else {
					s.DeliverableRetryNo = retry
				}
				got, e := Decide(s, kind, decision, Ptr("fix"), Ptr("edited"), 3)
				if e != nil {
					t.Fatal(e)
				}
				want := "executing"
				if kind == "deliverable" {
					want = "approved"
				}
				if decision == "reject" {
					want = "planning"
					if kind == "deliverable" {
						want = "executing"
					}
					if retry == 2 {
						want = "blocked"
					}
					if got.RetryCount != retry+1 || Text(got.LastFeedback) != "fix" {
						t.Fatal(got)
					}
				}
				if got.Status != want {
					t.Fatalf("%s %s %d: %+v", kind, decision, retry, got)
				}
				if decision == "edit" && ((kind == "plan" && Text(got.Plan) != "edited") || (kind == "deliverable" && Text(got.DeliverableSummary) != "edited")) {
					t.Fatal(got)
				}
			}
		}
	}
	for _, state := range []string{"pending", "planning", "executing", "approved", "blocked"} {
		if _, e := Decide(StageEntity{Status: state}, "plan", "approve", nil, nil, 3); e == nil {
			t.Fatal(state)
		}
	}
	for _, d := range []string{"reject", "edit", "unknown"} {
		if ValidateDecision(d, nil, nil) == nil {
			t.Fatal(d)
		}
	}
}
func TestDeriveParallelState(t *testing.T) {
	cases := []struct {
		states []string
		want   string
	}{{[]string{"approved", "plan_review"}, "waiting_approval"}, {[]string{"planning", "deliverable_review"}, "waiting_approval"}, {[]string{"approved", "executing"}, "running"}, {[]string{"approved", "approved"}, "completed"}, {[]string{"blocked", "approved"}, "blocked"}, {[]string{"blocked", "planning"}, "running"}}
	for _, c := range cases {
		stages := []StageEntity{}
		for _, s := range c.states {
			stages = append(stages, StageEntity{Status: s})
		}
		if got := Derive("running", stages); got != c.want {
			t.Fatalf("%v => %s", c.states, got)
		}
		for _, status := range []string{"paused", "completed", "cancelled", "failed"} {
			if Derive(status, stages) != status {
				t.Fatal(status)
			}
		}
	}
	stages := []StageEntity{{StageKey: "a", Status: "blocked"}, {StageKey: "b", Status: "pending", DependsOn: []string{"a"}}}
	if Derive("running", stages) != "blocked" {
		t.Fatal("blocked descendants counted as runnable")
	}
}
