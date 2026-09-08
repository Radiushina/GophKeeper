// Package blob stores ciphertext objects. The server never decrypts them.
package blob

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// ErrNotFound is a missing object.
var ErrNotFound = errors.New("blob not found")

// Store is object storage for encrypted file bodies.
type Store interface {
	Put(ctx context.Context, key string, data []byte) error
	Get(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, key string) error
	List(ctx context.Context, prefix string) ([]string, error)
}

// Mem is an in-memory Store for tests.
type Mem struct {
	mu   sync.Mutex
	data map[string][]byte
}

// NewMem builds an empty memory store.
func NewMem() *Mem {
	return &Mem{data: map[string][]byte{}}
}

// Put writes an object.
func (m *Mem) Put(_ context.Context, key string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = append([]byte(nil), data...)
	return nil
}

// Get reads an object.
func (m *Mem) Get(_ context.Context, key string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.data[key]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, key)
	}
	return append([]byte(nil), b...), nil
}

// Delete removes an object. Missing keys are ignored.
func (m *Mem) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
	return nil
}

// List returns keys with the given prefix.
func (m *Mem) List(_ context.Context, prefix string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []string
	for k := range m.data {
		if strings.HasPrefix(k, prefix) {
			out = append(out, k)
		}
	}
	return out, nil
}
