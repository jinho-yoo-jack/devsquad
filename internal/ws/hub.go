package ws

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/google/uuid"

	"github.com/jinho-yoo-jack/devsquad/internal/event"
	"github.com/jinho-yoo-jack/devsquad/internal/store"
)

type Hub struct {
	Store *store.Store
	Bus   *event.Bus
	mu    sync.Mutex
	conns map[*websocket.Conn]bool
}

func New(s *store.Store, b *event.Bus) *Hub {
	return &Hub{Store: s, Bus: b, conns: map[*websocket.Conn]bool{}}
}
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.conns {
		c.CloseNow()
	}
}

type message struct {
	Op      string  `json:"op"`
	TaskID  *string `json:"task_id"`
	FromSeq int64   `json:"from_seq"`
}
type reply struct {
	Op   string `json:"op"`
	Code string `json:"code,omitempty"`
}
type subscribed struct {
	Op       string `json:"op"`
	TaskID   string `json:"task_id,omitempty"`
	Replayed int    `json:"replayed"`
}

func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Local Phase 1 API: preserve the dashboard's separate localhost origin.
	c, e := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if e != nil {
		return
	}
	defer c.CloseNow()
	c.SetReadLimit(64 << 10)
	h.mu.Lock()
	h.conns[c] = true
	h.mu.Unlock()
	defer func() { h.mu.Lock(); delete(h.conns, c); h.mu.Unlock() }()
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	commands := make(chan message, 8)
	go func() {
		defer cancel()
		for {
			var m message
			if wsjson.Read(ctx, c, &m) != nil {
				return
			}
			select {
			case commands <- m:
			case <-ctx.Done():
				return
			}
		}
	}()
	sub := h.Bus.Subscribe("")
	defer sub.Close()
	// Overflow cancels a slow socket even while its replay is still being written.
	go func() {
		select {
		case <-sub.Done:
			cancel()
			c.CloseNow()
		case <-ctx.Done():
		}
	}()
	send := func(v any) error {
		timeout, done := context.WithTimeout(ctx, 5*time.Second)
		defer done()
		return wsjson.Write(timeout, c, v)
	}
	tasks := map[string]bool{}
	summary := false
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-sub.Events:
			if tasks[ev.TaskID] || (summary && (strings.HasPrefix(ev.Type, "run.") || strings.HasPrefix(ev.Type, "stage.") || strings.HasPrefix(ev.Type, "approval."))) {
				if send(ev) != nil {
					return
				}
			}
		case m := <-commands:
			if m.TaskID != nil {
				if _, e = uuid.Parse(*m.TaskID); e != nil {
					if send(reply{Op: "error", Code: "BAD_REQUEST"}) != nil {
						return
					}
					continue
				}
			}
			switch m.Op {
			case "ping":
				e = send(reply{Op: "pong"})
			case "unsubscribe":
				if m.TaskID == nil {
					summary = false
				} else {
					delete(tasks, *m.TaskID)
				}
			case "subscribe":
				if m.FromSeq < 0 {
					e = send(reply{Op: "error", Code: "BAD_REQUEST"})
					break
				}
				ack := subscribed{Op: "subscribed"}
				if m.TaskID == nil {
					summary = true
				} else {
					id := *m.TaskID
					t, err := h.Store.Task(ctx, id)
					if err != nil {
						e = send(reply{Op: "error", Code: "TASK_NOT_FOUND"})
						break
					}
					tasks[id] = true
					ack.TaskID = id
					after := m.FromSeq
					for after < t.EventSeq {
						batch, err := h.Store.Events(ctx, id, after, 500)
						if err != nil {
							_ = send(reply{Op: "error", Code: "REPLAY_FAILED"})
							return
						}
						if len(batch) == 0 {
							break
						}
						for _, ev := range batch {
							if ev.Seq > t.EventSeq {
								break
							}
							if send(ev) != nil {
								return
							}
							after = ev.Seq
							ack.Replayed++
						}
					}
				}
				e = send(ack)
			default:
				e = send(reply{Op: "error", Code: "UNKNOWN_OP"})
			}
			if e != nil {
				return
			}
		}
	}
}
