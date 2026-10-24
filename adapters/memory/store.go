package memory

import (
	"context"
	"sync"

	"github.com/beilunyang/openhongguo/sdk"
)

// Store keeps one anonymous profile in memory.
type Store struct {
	mu      sync.Mutex
	profile *sdk.DeviceProfile
}

func New() *Store { return &Store{} }

func (s *Store) Load(ctx context.Context) (*sdk.DeviceProfile, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.profile == nil {
		return nil, sdk.ErrProfileNotFound
	}
	cp := *s.profile
	return &cp, nil
}

func (s *Store) Save(ctx context.Context, profile *sdk.DeviceProfile) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if profile == nil {
		return sdk.ErrInvalidOptions
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *profile
	s.profile = &cp
	return nil
}

func (s *Store) Reset(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.profile = nil
	return nil
}
