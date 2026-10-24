package sdk

import "context"

// SignInput is the canonical request a Signer would cover.
type SignInput struct {
	CanonicalRequest []byte
}

// SignedHeaders are header names and values returned by a Signer.
type SignedHeaders map[string]string

// Signer produces request signatures. Phase 1 does not call a native signer.
type Signer interface {
	Sign(ctx context.Context, input SignInput) (SignedHeaders, error)
}

// UnavailableSigner reports that the requested signature is not portable pure Go.
type UnavailableSigner struct{}

func (UnavailableSigner) Sign(context.Context, SignInput) (SignedHeaders, error) {
	return nil, ErrSignerUnavailable
}
