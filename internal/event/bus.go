package event

import (
	"sync"

	"github.com/jinho-yoo-jack/devsquad/internal/domain"
)

type Subscription struct {
	Events <-chan domain.Event
	Done   <-chan struct{}
	Close  func()
}
type subscriber struct {
	task   string
	events chan domain.Event
	done   chan struct{}
}
type Bus struct {
	mu       sync.Mutex
	subs     map[*subscriber]bool
	Capacity int
}

func NewBus() *Bus { return &Bus{subs: map[*subscriber]bool{}, Capacity: 512} }
func (b *Bus) Subscribe(task string) Subscription {
	b.mu.Lock()
	defer b.mu.Unlock()
	s := &subscriber{task: task, events: make(chan domain.Event, max(1, b.Capacity)), done: make(chan struct{})}
	b.subs[s] = true
	return Subscription{s.events, s.done, func() { b.mu.Lock(); defer b.mu.Unlock(); b.remove(s) }}
}
func (b *Bus) remove(s *subscriber) {
	if b.subs[s] {
		delete(b.subs, s)
		close(s.done)
	}
}
func (b *Bus) Publish(ev domain.Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for s := range b.subs {
		if s.task != "" && s.task != ev.TaskID {
			continue
		}
		select {
		case s.events <- ev:
		default:
			b.remove(s)
		}
	}
}
