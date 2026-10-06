package orchestrator

import (
	"context"
	"testing"
	"testing/synctest"
	"time"
)

func TestRecoveryUsesFixedDelayAndStopsOnCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		calls := make(chan time.Time, 4)
		finished := make(chan struct{})
		start := time.Now()
		go func() {
			defer close(finished)
			recoveryLoop(ctx, 5*time.Minute, func() { calls <- time.Now(); time.Sleep(2 * time.Minute) })
		}()
		synctest.Wait()
		if got := <-calls; !got.Equal(start) {
			t.Fatal("initial recovery was delayed")
		}
		time.Sleep(6 * time.Minute)
		synctest.Wait()
		select {
		case <-calls:
			t.Fatal("used fixed-rate scheduling")
		default:
		}
		time.Sleep(time.Minute)
		synctest.Wait()
		if got := <-calls; got.Sub(start) != 7*time.Minute {
			t.Fatal(got.Sub(start))
		}
		cancel()
		time.Sleep(2 * time.Minute)
		synctest.Wait()
		<-finished
	})
}
