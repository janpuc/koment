package session

import (
	"errors"
	"sync"
)

var ErrNotFound = errors.New("no such session")

type Store struct {
	mu       sync.RWMutex
	sessions map[string]Token
}

func NewStore() *Store {
	return &Store{sessions: make(map[string]Token)}
}

func (s *Store) Put(id string, token Token) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[id] = token
}

func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.sessions)
}

func (s *Store) Get(id string) (Token, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	token, ok := s.sessions[id]
	if !ok {
		return Token{}, ErrNotFound
	}
	return token, nil
}
