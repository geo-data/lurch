package main

import "github.com/fsouza/go-dockerclient"

type Config struct {
	BotName        string
	CommandChannel string
	SlackToken     string
	Docker         dockerConfig
	UpdateImage    bool
	Debug          bool
}

type dockerConfig struct {
	Image string
	Tag   string
	Auth  docker.AuthConfiguration
}
