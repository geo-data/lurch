package main

import "sync"

type DeployState struct {
	sync.Mutex
	state map[string]bool
}

func NewDeployState() (s *DeployState) {
	s = &DeployState{}
	s.state = make(map[string]bool)
	return
}

func (s *DeployState) Set(key string) bool {
	s.Lock()
	defer s.Unlock()
	if _, ok := s.state[key]; ok {
		return false
	}

	s.state[key] = true
	return true
}

func (s *DeployState) Unset(key string) bool {
	s.Lock()
	defer s.Unlock()
	if _, ok := s.state[key]; !ok {
		return false
	}

	delete(s.state, key)
	return true
}
