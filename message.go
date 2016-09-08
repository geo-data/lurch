package main

import (
	"strings"

	"github.com/nlopes/slack"
)

type Message struct {
	Text string
	rtm  *slack.RTM
	ev   *slack.MessageEvent
}

func NewMessage(rtm *slack.RTM, ev *slack.MessageEvent, user *User) *Message {
	var prefix string

	if strings.HasPrefix(ev.Text, user.Name) {
		prefix = user.Name
	} else if strings.HasPrefix(ev.Text, user.Mention) {
		prefix = user.Mention
	} else {
		return nil // The message isn't for the user.
	}

	return &Message{
		Text: strings.Trim(strings.TrimPrefix(ev.Text, prefix), ": "),
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

func (msg *Message) Reply(reply string) {
	msg.rtm.SendMessage(msg.rtm.NewOutgoingMessage(reply, msg.ev.Channel))
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
