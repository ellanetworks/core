package server

import "sync"

// JWTSecret holds the JWT signing secret with thread-safe access,
// supporting runtime rotation via the rotate-secret endpoint.
type JWTSecret struct {
	mu     sync.RWMutex
	secret []byte
}

func NewJWTSecret(secret []byte) *JWTSecret {
	return &JWTSecret{secret: secret}
}

func (s *JWTSecret) Get() []byte {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cp := make([]byte, len(s.secret))
	copy(cp, s.secret)

	return cp
}

func (s *JWTSecret) Set(secret []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.secret = secret
}
