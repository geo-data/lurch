package main

// This provides utilities for dealing with slack.

import "github.com/nlopes/slack"

func GetChannelByName(api *slack.Client, name string) (channel *slack.Channel, err error) {
	var channels []slack.Channel
	if channels, err = api.GetChannels(true); err != nil {
		return
	}
	for _, c := range channels {
		if c.Name == name {
			channel = &c
			return
		}
	}

	return
}

func GetGroupByName(api *slack.Client, name string) (group *slack.Group, err error) {
	var groups []slack.Group
	if groups, err = api.GetGroups(true); err != nil {
		return
	}
	for _, g := range groups {
		if g.Name == name {
			group = &g
			return
		}
	}

	return
}

func GetIDFromName(api *slack.Client, name string) (id string, err error) {
	var (
		channel *slack.Channel
		group   *slack.Group
	)

	if channel, err = GetChannelByName(api, name); err != nil {
		return
	} else if channel != nil {
		id = channel.ID
		return
	}

	if group, err = GetGroupByName(api, name); err != nil {
		return
	} else if group != nil {
		id = group.ID
		return
	}

	return
}
