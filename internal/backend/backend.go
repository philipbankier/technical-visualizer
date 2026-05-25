package backend

import (
	"context"
	"time"
)

const (
	defaultOperationTimeout       = 30 * time.Second
	defaultImageGenerationTimeout = 5 * time.Minute
)

type ImageRequest struct {
	Prompt       string
	ScaffoldHTML string
	OutputPath   string
	Size         string
	Quality      string
}

type ImageBackend interface {
	Name() string
	Available(context.Context) Capability
	Generate(context.Context, ImageRequest) error
}

type Capability struct {
	Available bool
	Name      string
	Reason    string
	Remote    bool
}

func operationContext(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		timeout = defaultOperationTimeout
	}
	deadline := time.Now().Add(timeout)
	if existing, ok := ctx.Deadline(); ok && existing.Before(deadline) {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, timeout)
}
