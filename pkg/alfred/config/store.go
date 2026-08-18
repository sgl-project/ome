package config

import (
	"sync/atomic"
)

// Reload outcomes, used as the alfred_policy_reload_total outcome label.
const (
	OutcomeSuccess = "success"
	OutcomeFailure = "failure"
)

// Store holds the active configuration with last-known-good semantics: a
// failed Update leaves the previous config serving. Reads are lock-free and
// safe from any goroutine; the returned *Config is shared and must be treated
// as immutable.
type Store struct {
	current atomic.Pointer[Config]
}

// NewStore returns a store serving the built-in defaults until the first
// successful Update — Alfred starts safe (recommend-only) even if the
// ConfigMap is missing or broken at boot.
func NewStore() *Store {
	s := &Store{}
	s.current.Store(Default())
	return s
}

// Get returns the active configuration.
func (s *Store) Get() *Config {
	return s.current.Load()
}

// Update parses and validates raw config.yaml content. On success the new
// config becomes active and OutcomeSuccess is returned; on failure the
// previous config stays active and OutcomeFailure is returned with the
// validation error.
func (s *Store) Update(raw []byte) (string, error) {
	cfg, err := Load(raw)
	if err != nil {
		return OutcomeFailure, err
	}
	s.current.Store(cfg)
	return OutcomeSuccess, nil
}
