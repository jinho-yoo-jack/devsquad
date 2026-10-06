package scm

import "context"

type Draft struct {
	TaskID, Title, Body string
	Paths               []string
}
type Result struct {
	URL       string
	Published bool
}
type Publisher interface {
	Publish(context.Context, Draft) (Result, error)
}
type Noop struct{}

func (Noop) Publish(context.Context, Draft) (Result, error) { return Result{Published: false}, nil }
