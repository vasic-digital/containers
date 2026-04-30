package event

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultEventBus_PublishSubscribe(t *testing.T) {
	bus := NewEventBus(16)
	defer bus.Close()

	var received Event
	var wg sync.WaitGroup
	wg.Add(1)

	bus.Subscribe(
		EventFilter{},
		func(_ context.Context, ev Event) {
			received = ev
			wg.Done()
		},
	)

	ev := NewEvent(EventContainerStarted, "runtime", "pg")
	bus.Publish(context.Background(), ev)

	waitWithTimeout(t, &wg, 2*time.Second)
	assert.Equal(t, EventContainerStarted, received.Type)
	assert.Equal(t, "pg", received.Name)
}

func TestDefaultEventBus_FilterByType(t *testing.T) {
	bus := NewEventBus(16)
	defer bus.Close()

	var count atomic.Int32

	bus.Subscribe(
		EventFilter{Types: []EventType{EventHealthChanged}},
		func(_ context.Context, _ Event) {
			count.Add(1)
		},
	)

	// Allow subscriber goroutine to start.
	time.Sleep(10 * time.Millisecond)

	ctx := context.Background()
	bus.Publish(ctx, NewEvent(
		EventContainerStarted, "r", "a",
	))
	bus.Publish(ctx, NewEvent(
		EventHealthChanged, "r", "b",
	))
	bus.Publish(ctx, NewEvent(
		EventContainerStopped, "r", "c",
	))
	bus.Publish(ctx, NewEvent(
		EventHealthChanged, "r", "d",
	))

	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, int32(2), count.Load())
}

func TestDefaultEventBus_FilterBySource(t *testing.T) {
	bus := NewEventBus(16)
	defer bus.Close()

	var count atomic.Int32

	bus.Subscribe(
		EventFilter{Sources: []string{"compose"}},
		func(_ context.Context, _ Event) {
			count.Add(1)
		},
	)

	time.Sleep(10 * time.Millisecond)

	ctx := context.Background()
	bus.Publish(ctx, NewEvent(
		EventBootStarted, "compose", "x",
	))
	bus.Publish(ctx, NewEvent(
		EventBootStarted, "runtime", "y",
	))

	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, int32(1), count.Load())
}

func TestDefaultEventBus_Unsubscribe(t *testing.T) {
	bus := NewEventBus(16)
	defer bus.Close()

	var count atomic.Int32

	id := bus.Subscribe(
		EventFilter{},
		func(_ context.Context, _ Event) {
			count.Add(1)
		},
	)

	time.Sleep(10 * time.Millisecond)

	ctx := context.Background()
	bus.Publish(ctx, NewEvent(EventBootStarted, "s", "n"))
	time.Sleep(50 * time.Millisecond)

	bus.Unsubscribe(id)
	time.Sleep(10 * time.Millisecond)

	bus.Publish(ctx, NewEvent(EventBootCompleted, "s", "n"))
	time.Sleep(50 * time.Millisecond)

	assert.Equal(t, int32(1), count.Load())
}

func TestDefaultEventBus_MultipleSubscribers(t *testing.T) {
	// bluff-scan: no-assert-ok (event-bus smoke — pub/sub must not panic on any subscriber count)
	bus := NewEventBus(16)
	defer bus.Close()

	var wg sync.WaitGroup
	wg.Add(3)

	for i := 0; i < 3; i++ {
		bus.Subscribe(
			EventFilter{},
			func(_ context.Context, _ Event) {
				wg.Done()
			},
		)
	}

	bus.Publish(
		context.Background(),
		NewEvent(EventBootStarted, "s", "n"),
	)

	waitWithTimeout(t, &wg, 2*time.Second)
}

func TestDefaultEventBus_Close(t *testing.T) {
	bus := NewEventBus(16)

	var wg sync.WaitGroup
	wg.Add(1)

	bus.Subscribe(
		EventFilter{},
		func(_ context.Context, _ Event) {
			wg.Done()
		},
	)

	bus.Publish(
		context.Background(),
		NewEvent(EventBootStarted, "s", "n"),
	)
	waitWithTimeout(t, &wg, 2*time.Second)

	bus.Close()
	// Publish after close should not panic.
	bus.Publish(
		context.Background(),
		NewEvent(EventBootCompleted, "s", "n"),
	)
}

func TestDefaultEventBus_DoubleClose(t *testing.T) {
	bus := NewEventBus(16)
	bus.Close()
	// Must not panic on second close.
	bus.Close()
}

func TestNewEventBus_NegativeBuffer(t *testing.T) {
	bus := NewEventBus(-1)
	require.NotNil(t, bus)
	bus.Close()
}

func TestSubscriptionID_Uniqueness(t *testing.T) {
	bus := NewEventBus(4)
	defer bus.Close()

	handler := func(_ context.Context, _ Event) {}
	id1 := bus.Subscribe(EventFilter{}, handler)
	id2 := bus.Subscribe(EventFilter{}, handler)
	assert.NotEqual(t, id1, id2)
}

// waitWithTimeout waits for a WaitGroup with a deadline.
func waitWithTimeout(
	t *testing.T,
	wg *sync.WaitGroup,
	timeout time.Duration,
) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
		t.Fatal("timed out waiting for WaitGroup")
	}
}
