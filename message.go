package main

import (
	"fmt"
	"strings"

	"github.com/nlopes/slack"
)

type Message struct {
	Text string
	rtm  *slack.RTM
	ev   *slack.MessageEvent
}

func NewMessage(rtm *slack.RTM, ev *slack.MessageEvent, user *User) *Message {
	var prefix, text string

	// Don't respond to myself.
	if ev.User == user.ID {
		return nil
	}

	// Get the message text.
	if ev.SubMessage != nil && ev.SubType == "message_changed" {
		text = ev.SubMessage.Text
	} else {
		text = ev.Text
	}

	// Decide if the message is for us.
	if strings.HasPrefix(text, user.Name) {
		var reply string
		if ev.User != "" {
			reply = fmt.Sprintf("<@%s> are you talking to me?", ev.User)
		} else {
			reply = "Are you talking to me?"
		}
		reply += fmt.Sprintf(" Please mention me directly as %s.", user.Mention)
		sendMessage(reply, rtm, ev)
		return nil
	} else if strings.HasPrefix(text, user.Mention) {
		prefix = user.Mention
	} else {
		return nil // The message isn't for the user.
	}

	return &Message{
		Text: strings.Trim(strings.TrimPrefix(text, prefix), ": "),
		rtm:  rtm,
		ev:   ev,
	}
}

// Command returns a string list of component commands specified by msg.Text.
func (msg *Message) Command() (cmd []string) {
	parts := strings.Split(msg.Text, " ")
	for _, val := range parts {
		if val != "" { // Skip empty commands.
			cmd = append(cmd, val)
		}
	}

	return
}

func sendMessage(msg string, rtm *slack.RTM, ev *slack.MessageEvent) {
	rtm.SendMessage(rtm.NewOutgoingMessage(msg, ev.Channel))
}

func (msg *Message) Reply(reply string) {
	sendMessage(reply, msg.rtm, msg.ev)
}

// Send implements the Conversation interface.
func (msg *Message) Send(message string) error {
	msg.Reply(message)
	return nil
}

type Conversation interface {
	Send(msg string) error
}

type ChannelMessage struct {
	rtm       *slack.RTM
	channelID string
}

func NewChannelMessage(rtm *slack.RTM, channelID string) *ChannelMessage {
	return &ChannelMessage{
		rtm:       rtm,
		channelID: channelID,
	}
}

// Send implements the Conversation interface.
func (m *ChannelMessage) Send(msg string) error {
	m.rtm.SendMessage(m.rtm.NewOutgoingMessage(msg, m.channelID))
	return nil
}
