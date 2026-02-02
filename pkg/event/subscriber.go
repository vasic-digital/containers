package event

import (
	"context"
	"fmt"
)

// deliverableEvent pairs an event with the context active at the
// time of publishing.
type deliverableEvent struct {
	ctx   context.Context
	event Event
}

// subscription holds the state of a single event subscriber.
type subscription struct {
	id      SubscriptionID
	filter  EventFilter
	handler EventHandler
	ch      chan deliverableEvent
	done    chan struct{}
}

// newSubscriptionID creates a SubscriptionID from a uint64.
func newSubscriptionID(n uint64) SubscriptionID {
	return SubscriptionID(fmt.Sprintf("sub-%d", n))
}

// newSubscription creates a subscription ready to be started.
func newSubscription(
	id SubscriptionID,
	filter EventFilter,
	handler EventHandler,
	bufferSize int,
) *subscription {
	return &subscription{
		id:      id,
		filter:  filter,
		handler: handler,
		ch:      make(chan deliverableEvent, bufferSize),
		done:    make(chan struct{}),
	}
}

// run processes events from the channel until stop is called or
// closeCh is closed.
func (s *subscription) run(closeCh <-chan struct{}) {
	defer close(s.done)
	for {
		select {
		case de, ok := <-s.ch:
			if !ok {
				return
			}
			s.handler(de.ctx, de.event)
		case <-closeCh:
			return
		}
	}
}

// stop closes the event channel and waits for the goroutine to
// finish.
func (s *subscription) stop() {
	close(s.ch)
	<-s.done
}

// matches returns true if the event satisfies the subscription's
// filter criteria.
func (s *subscription) matches(e Event) bool {
	if len(s.filter.Types) > 0 {
		found := false
		for _, t := range s.filter.Types {
			if t == e.Type {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if len(s.filter.Sources) > 0 {
		found := false
		for _, src := range s.filter.Sources {
			if src == e.Source {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
