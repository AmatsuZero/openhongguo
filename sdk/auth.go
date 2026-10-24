package sdk

import "context"

// AuthProvider may attach account state. Phase 1 only ships the anonymous provider.
type AuthProvider interface {
	Apply(ctx context.Context, req *Request) error
}

// AnonymousProvider adds no account credentials.
type AnonymousProvider struct{}

func (AnonymousProvider) Apply(context.Context, *Request) error { return nil }
