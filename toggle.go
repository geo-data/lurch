package main

import "sync"

type Toggle struct {
	m  sync.Mutex
	on bool
}

func (t *Toggle) IsOn() bool {
	t.m.Lock()
	defer t.m.Unlock()
	return t.on
}

func (t *Toggle) On() {
	t.m.Lock()
	defer t.m.Unlock()
	t.on = true
	return
}

func (t *Toggle) Off() {
	t.m.Lock()
	defer t.m.Unlock()
	t.on = false
	return
}
