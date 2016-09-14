package main

import (
	"sort"
	"sync"

	"github.com/fsouza/go-dockerclient"
)

type Config struct {
	sync.RWMutex

	Channels     *Channels // Channels of which Lurch is a member.
	BotName      string
	EnableDM     bool
	SlackToken   string
	Docker       dockerConfig
	UpdateImage  bool
	Debug        bool
	ConnAttempts int
	Stacks       map[string]Stack
}

// GetStackList returns an ordered list of stack names.
func (c *Config) GetStackList() (stacks []string) {
	for stack := range c.Stacks {
		stacks = append(stacks, stack)
	}
	sort.Strings(stacks)
	return
}

type Channels struct {
	sync.RWMutex
	Names map[string]bool
}

func NewChannels() *Channels {
	return &Channels{
		Names: make(map[string]bool),
	}
}

func (c *Channels) RemoveChannel(id string) {
	c.Lock()
	defer c.Unlock()
	delete(c.Names, id)
}

func (c *Channels) AddChannel(id string) {
	c.Lock()
	defer c.Unlock()
	c.Names[id] = true
}

func (c *Channels) HasChannel(id string) (yes bool) {
	c.RLock()
	defer c.RUnlock()
	_, yes = c.Names[id]
	return
}

func (c *Channels) GetChannels() (chans []string) {
	for id := range c.Names {
		chans = append(chans, id)
	}
	return
}

type Stack struct {
	Playbooks map[string]Playbook `yaml:",inline"`
}

// GetPlaybookList returns an ordered list of playbook names.
func (s Stack) GetPlaybookList() (playbooks []string) {
	for playbook := range s.Playbooks {
		playbooks = append(playbooks, playbook)
	}
	sort.Strings(playbooks)
	return
}

type Playbook struct {
	Location string            `yaml:"playbook"`
	About    string            `yaml:"about"`
	Actions  map[string]Action `yaml:"actions,omitempty"`
}

// GetActionList returns an ordered list of action names.
func (p Playbook) GetActionList() (actions []string) {
	for action := range p.Actions {
		actions = append(actions, action)
	}
	sort.Strings(actions)
	return
}

type Action struct {
	About string            `yaml:"about"`
	Vars  map[string]string `yaml:"vars,omitempty"`
}

type dockerConfig struct {
	Image string
	Tag   string
	Auth  docker.AuthConfiguration
}
