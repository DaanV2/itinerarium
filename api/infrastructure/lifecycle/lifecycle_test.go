package lifecycle_test

import (
	"context"
	"errors"
	"testing"

	"github.com/DaanV2/itinerarium/api/infrastructure/lifecycle"
	"github.com/stretchr/testify/require"
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
	require.ErrorIs(t, err, b.err, "b's error should be joined")

	want := []string{
		"a.before", "b.before",
		"a.shutdown", "b.shutdown",
		"a.after", "b.after",
		"a.cleanup", "b.cleanup",
	}
	require.Equal(t, want, calls)
}

func TestShutdownAllIgnoresPlainComponents(t *testing.T) {
	require.NoError(t, lifecycle.ShutdownAll(context.Background(), struct{}{}, nil))
}
