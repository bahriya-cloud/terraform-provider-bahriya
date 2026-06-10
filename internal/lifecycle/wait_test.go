package lifecycle

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWaitForStatus_ReachesTarget(t *testing.T) {
	calls := 0
	get := func(ctx context.Context) (string, error) {
		calls++
		if calls < 3 {
			return "PROVISIONING", nil
		}
		return "RUNNING", nil
	}
	s, err := WaitForStatus(context.Background(), get, WaitOptions{
		Target:   "RUNNING",
		Timeout:  2 * time.Second,
		Interval: 5 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s != "RUNNING" {
		t.Fatalf("got %q want RUNNING", s)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestWaitForStatus_HitsTerminal(t *testing.T) {
	get := func(ctx context.Context) (string, error) {
		return "ERROR", nil
	}
	s, err := WaitForStatus(context.Background(), get, WaitOptions{
		Target:   "RUNNING",
		Terminal: []string{"ERROR", "FAILED"},
		Timeout:  1 * time.Second,
		Interval: 5 * time.Millisecond,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if s != "ERROR" {
		t.Fatalf("got %q want ERROR", s)
	}
}

func TestWaitForStatus_TimesOut(t *testing.T) {
	get := func(ctx context.Context) (string, error) {
		return "PROVISIONING", nil
	}
	s, err := WaitForStatus(context.Background(), get, WaitOptions{
		Target:   "RUNNING",
		Timeout:  20 * time.Millisecond,
		Interval: 5 * time.Millisecond,
	})
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if s != "PROVISIONING" {
		t.Fatalf("got %q want PROVISIONING", s)
	}
}

func TestWaitForStatus_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	get := func(ctx context.Context) (string, error) {
		cancel()
		return "PROVISIONING", nil
	}
	_, err := WaitForStatus(ctx, get, WaitOptions{
		Target:   "RUNNING",
		Timeout:  1 * time.Second,
		Interval: 5 * time.Millisecond,
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestWaitForStatus_PropagatesGetError(t *testing.T) {
	want := errors.New("network down")
	get := func(ctx context.Context) (string, error) {
		return "", want
	}
	_, err := WaitForStatus(context.Background(), get, WaitOptions{
		Target:   "RUNNING",
		Timeout:  1 * time.Second,
		Interval: 5 * time.Millisecond,
	})
	if !errors.Is(err, want) {
		t.Fatalf("expected %v, got %v", want, err)
	}
}
