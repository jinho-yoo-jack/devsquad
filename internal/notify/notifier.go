package notify

import (
	"context"
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/jinho-yoo-jack/devsquad/internal/domain"
	"github.com/jinho-yoo-jack/devsquad/internal/event"
)

type Notifier interface {
	Notify(context.Context, domain.Event) error
}
type Noop struct{}

func (Noop) Notify(context.Context, domain.Event) error { return nil }
func Run(ctx context.Context, bus *event.Bus, n Notifier, failures prometheus.Counter) {
	sub := bus.Subscribe("")
	defer sub.Close()
	for {
		select {
		case <-ctx.Done():
			return
		case <-sub.Done:
			failures.Inc()
			slog.Error("[Notifier] Subscription overflow")
			return
		case ev := <-sub.Events:
			if e := n.Notify(ctx, ev); e != nil {
				failures.Inc()
				slog.Error("[Notifier] Notify failed", "event_id", ev.EventID, "error", e)
			}
		}
	}
}
