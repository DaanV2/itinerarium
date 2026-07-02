package lifecycle_test

import (
	"context"
	"errors"
	"testing"

	"github.com/DaanV2/itinerarium/api/infrastructure/lifecycle"
)

type recorder struct {
	calls *[]string
	name  string
	err   error
}

func (r *recorder) BeforeShutdown() { *r.calls = append(*r.calls, r.name+".before") }

func (r *recorder) Shutdown(_ context.Context) error {
	*r.calls = append(*r.calls, r.name+".shutdown")

	return r.err
}

func (r *recorder) AfterShutDown() { *r.calls = append(*r.calls, r.name+".after") }

func (r *recorder) ShutdownCleanup() error {
	*r.calls = append(*r.calls, r.name+".cleanup")

	return nil
}

func TestShutdownAllRunsPhasesInOrder(t *testing.T) {
	var calls []string

	a := &recorder{calls: &calls, name: "a"}
	b := &recorder{calls: &calls, name: "b", err: errors.New("b failed")}

	err := lifecycle.ShutdownAll(context.Background(), a, b)
	if err == nil || !errors.Is(err, b.err) {
		t.Fatalf("expected b's error to be joined, got %v", err)
	}

	want := []string{
		"a.before", "b.before",
		"a.shutdown", "b.shutdown",
		"a.after", "b.after",
		"a.cleanup", "b.cleanup",
	}
	if len(calls) != len(want) {
		t.Fatalf("expected %d calls, got %v", len(want), calls)
	}

	for i, w := range want {
		if calls[i] != w {
			t.Fatalf("call %d: expected %q, got %q (all: %v)", i, w, calls[i], calls)
		}
	}
}

func TestShutdownAllIgnoresPlainComponents(t *testing.T) {
	if err := lifecycle.ShutdownAll(context.Background(), struct{}{}, nil); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}
