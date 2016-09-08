package main

import (
	"sort"

	"github.com/fsouza/go-dockerclient"
)

type Config struct {
	BotName        string
	CommandChannel string
	SlackToken     string
	Docker         dockerConfig
	UpdateImage    bool
	Debug          bool
	Stacks         map[string]Stack
}

// GetStackList returns an ordered list of stack names.
func (c *Config) GetStackList() (stacks []string) {
	for stack := range c.Stacks {
		stacks = append(stacks, stack)
	}
	sort.Strings(stacks)
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
