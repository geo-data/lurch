package main

import (
	"fmt"

	"github.com/nlopes/slack"
)

type User struct {
	Name,
	Mention string // The string matching @ mentions for this user.
	ID string
}

func NewUser(user *slack.UserDetails) *User {
	return &User{
		Name:    user.Name,
		Mention: fmt.Sprintf("<@%s>", user.ID),
		ID:      user.ID,
	}
}
