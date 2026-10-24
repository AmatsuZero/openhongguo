package file

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/beilunyang/openhongguo/sdk"
)

// ProfileStore saves the anonymous profile with mode 0600.
type ProfileStore struct {
	path string
	mu   sync.Mutex
}

func NewProfileStore(path string) *ProfileStore { return &ProfileStore{path: path} }

func (s *ProfileStore) Load(ctx context.Context) (*sdk.DeviceProfile, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := rejectSymlink(s.path); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, sdk.ErrProfileNotFound
	}
	if err != nil {
		return nil, err
	}
	var p sdk.DeviceProfile
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *ProfileStore) Save(ctx context.Context, p *sdk.DeviceProfile) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if p == nil {
		return sdk.ErrInvalidOptions
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := rejectSymlink(s.path); err != nil {
		return err
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".ohg-profile-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	if err := json.NewEncoder(tmp).Encode(p); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(name, s.path); err != nil {
		return err
	}
	return os.Chmod(s.path, 0600)
}

func (s *ProfileStore) Reset(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := rejectSymlink(s.path); err != nil {
		return err
	}
	err := os.Remove(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func rejectSymlink(path string) error {
	if path == "" {
		return sdk.ErrInvalidOptions
	}
	info, err := os.Lstat(path)
	if err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("openhongguo: symlink profile path")
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
