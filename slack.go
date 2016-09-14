package main

// This provides utilities for dealing with slack.

import "github.com/nlopes/slack"

func BroadcastMessage(rtm *slack.RTM, msg string, channels []string) {
	for _, id := range channels {
		rtm.SendMessage(rtm.NewOutgoingMessage(msg, id))
	}
}
