package main

import "sync"

type RunState struct {
	sync.Mutex
	state map[string]bool
}

func NewRunState() (s *RunState) {
	s = &RunState{}
	s.state = make(map[string]bool)
	return
}

func (s *RunState) Set(key string) bool {
	s.Lock()
	defer s.Unlock()
	if _, ok := s.state[key]; ok {
		return false
	}

	s.state[key] = true
	return true
}

func (s *RunState) Unset(key string) bool {
	s.Lock()
	defer s.Unlock()
	if _, ok := s.state[key]; !ok {
		return false
	}

	delete(s.state, key)
	return true
}
