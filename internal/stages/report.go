package stages

import (
	"context"
	"fmt"

	"github.com/mattjoyce/framore/internal/batch"
	"github.com/mattjoyce/framore/internal/pipeline"
)

type Report struct{}

func (r *Report) Name() string { return "report" }

func (r *Report) Enabled(b *batch.Batch) bool { return b.Stages.Report }

func (r *Report) Run(ctx context.Context, b *batch.Batch, results *pipeline.Results) error {
	fmt.Println("  [report] report generation is not yet implemented")
	return nil
}
