package pipeline

import (
	"context"
	"testing"

	"github.com/mattjoyce/framore/internal/batch"
)

// fakeStage is a minimal Stage implementation for runner tests.
type fakeStage struct {
	name     string
	supports bool
	ran      bool
	runErr   error
}

func (f *fakeStage) Name() string                { return f.name }
func (f *fakeStage) Enabled(_ *batch.Batch) bool { return true }
func (f *fakeStage) SupportsNoWait() bool        { return f.supports }
func (f *fakeStage) Run(_ context.Context, _ *batch.Batch, _ *Results) error {
	f.ran = true
	return f.runErr
}

func TestRunSkipsIncompatibleStagesUnderNoWait(t *testing.T) {
	compatible := &fakeStage{name: "compatible", supports: true}
	incompatible := &fakeStage{name: "incompatible", supports: false}

	stages := []Stage{compatible, incompatible}
	b := &batch.Batch{}

	if _, err := Run(context.Background(), b, stages, false, true); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !compatible.ran {
		t.Error("compatible stage should have run under --no-wait")
	}
	if incompatible.ran {
		t.Error("incompatible stage should have been skipped under --no-wait")
	}
}

func TestRunWithoutNoWaitRunsAllEnabledStages(t *testing.T) {
	a := &fakeStage{name: "a", supports: true}
	b := &fakeStage{name: "b", supports: false}

	stages := []Stage{a, b}
	batchObj := &batch.Batch{}

	if _, err := Run(context.Background(), batchObj, stages, false, false); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !a.ran {
		t.Error("stage a should have run")
	}
	if !b.ran {
		t.Error("stage b should have run (incompatibility only matters under --no-wait)")
	}
}
