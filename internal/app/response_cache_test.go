package app

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCoalescedResponseSurvivesInitiatingRequestCancellation(t *testing.T) {
	lifetime, cancelLifetime := context.WithTimeout(t.Context(), time.Second)
	defer cancelLifetime()
	s := &service{ctx: lifetime}
	request, cancelRequest := context.WithCancel(lifetime)
	defer cancelRequest()
	started, release := make(chan struct{}), make(chan struct{})
	first := make(chan error, 1)
	go func() {
		_, err := s.coalesceResponse(request, "key", func(ctx context.Context) ([]byte, error) {
			close(started)
			select {
			case <-release:
				return []byte("snapshot result"), ctx.Err()
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		})
		first <- err
	}()
	select {
	case <-started:
	case <-lifetime.Done():
		t.Fatal("shared work did not start")
	}

	// Register an identical waiter synchronously so it is in the same flight
	// before canceling the request that originally started the work.
	second := s.flight.DoChan("key", func() (any, error) {
		return nil, errors.New("duplicate work")
	})
	cancelRequest()
	select {
	case err := <-first:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-lifetime.Done():
		t.Fatal("canceled waiter did not return while work was blocked")
	}
	close(release)
	select {
	case result := <-second:
		if result.Err != nil || !result.Shared || string(result.Val.([]byte)) != "snapshot result" {
			t.Fatalf("other waiters lost the shared response: %+v", result)
		}
	case <-lifetime.Done():
		t.Fatal("shared result never arrived")
	}
}
