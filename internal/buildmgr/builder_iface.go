package buildmgr

import (
	"context"

	"github.com/glennswest/rosecicd/internal/builder"
)

// Builder is the interface for build execution backends.
type Builder interface {
	Name() string
	Arch() string
	Run(ctx context.Context, spec builder.BuildSpec, buildID string) (logs string, err error)
	Healthy() bool
}
