package event

import (
	"testing"

	"github.com/jinho-yoo-jack/devsquad/internal/domain"
)

func TestSlowSubscriberIsDisconnectedWithoutBlockingOthers(t *testing.T) {
	b := NewBus()
	b.Capacity = 1
	slow := b.Subscribe("a")
	defer slow.Close()
	fast := b.Subscribe("a")
	defer fast.Close()
	other := b.Subscribe("b")
	defer other.Close()
	b.Publish(domain.Event{TaskID: "a", Seq: 1})
	<-fast.Events
	b.Publish(domain.Event{TaskID: "a", Seq: 2})
	select {
	case <-slow.Done:
	default:
		t.Fatal("slow subscriber not disconnected")
	}
	if ev := <-fast.Events; ev.Seq != 2 {
		t.Fatal(ev)
	}
	select {
	case <-other.Events:
		t.Fatal("event escaped task filter")
	default:
	}
}
