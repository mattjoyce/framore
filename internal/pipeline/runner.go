package pipeline

import (
	"context"
	"fmt"

	"github.com/mattjoyce/framore/internal/batch"
)

func Run(ctx context.Context, b *batch.Batch, stages []Stage, verbose, noWait bool) (*Results, error) {
	results := NewResults()
	for _, s := range stages {
		if !s.Enabled(b) {
			if verbose {
				fmt.Printf("[%s] skipped (disabled)\n", s.Name())
			}
			continue
		}
		if noWait && !s.SupportsNoWait() {
			fmt.Printf("[%s] skipped — not compatible with --no-wait\n", s.Name())
			continue
		}
		fmt.Printf("[%s] starting…\n", s.Name())
		if err := s.Run(ctx, b, results); err != nil {
			fmt.Printf("[%s] error: %v\n", s.Name(), err)
			// continue with next stage — don't abort
			continue
		}
		fmt.Printf("[%s] done\n", s.Name())
	}
	return results, nil
}
