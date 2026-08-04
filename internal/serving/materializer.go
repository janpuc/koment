package serving

import (
	"context"

	"github.com/janpuc/koment/internal/store"
)

type Materialization struct {
	Branch      string
	Commit      string
	PullRequest int
	URL         string
}

type Materializer interface {
	Materialize(context.Context, Repository, string, store.Annotation) (Materialization, error)
}
