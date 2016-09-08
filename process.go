package main

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/fsouza/go-dockerclient"
	"github.com/nlopes/slack"
	yaml "gopkg.in/yaml.v2"
)

var pulling Toggle

func sendHelp(intro string, msg *Message) {
	msg.Reply(fmt.Sprintf(`%s. I can help with the following commands:
• *%s* - deploy an application.
• *%s* - list applications I can deploy.
Use *%s* for further details.`,
		intro,
		"`deploy`",
		"`list`",
		"`help <command>`",
	))
}

func helpList(intro string, msg *Message) {
	var reply string
	if len(intro) > 0 {
		reply = fmt.Sprintf("%s. ", intro)
	}
	reply += "Use *`list`* as follows:\n  • Simply *`list`* to find the projects I can deal with;\n  • *`list <project>`* to find playbooks associated with a project;\n  • and *`list <project> <playbook>`* to describe any custom actions available for a playbook."
	msg.Reply(reply)
}

func processHelp(msg *Message, cmd []string) {
	if len(cmd) == 0 {
		sendHelp("Hmmm", msg)
		return
	}

	switch cmd[0] {
	case "deploy":
		msg.Reply("Use *`deploy <project> <service>`* to deploy a service related to a project. If a project has custom actions associated with it then just replace `deploy` with the name of the action.")
	case "list":
		helpList("", msg)
	default:
		sendHelp("I am a simple entity: I don't understand.", msg)
	}
}

// Desentence trims text of whitespace, lowercases the initial character and
// removes any trailing period, returning the result.
func Desentence(s string) string {
	if s == "" {
		return s
	}

	s = strings.TrimSuffix(strings.TrimSpace(s), ".")
	return strings.ToLower(string([]rune(s)[0])) + s[1:]
}

func listProjects(msg *Message, config *Config) {
	projects := config.GetStackList()

	var reply string
	switch len(projects) {
	case 0:
		reply = "Sorry, there don't seem to be any projects at the moment."
	case 1:
		reply = fmt.Sprintf("I only know about the *%s* project.", projects[0])
	default:
		reply = fmt.Sprintf("I know about the following %d projects:\n  • %s", len(projects), strings.Join(projects, "\n  • "))
	}

	msg.Reply(reply)
	return
}

func getStack(msg *Message, project string, config *Config) (stack *Stack) {
	if s, ok := config.Stacks[project]; !ok {
		msg.Reply(fmt.Sprintf("Oh dear.  I'm afraid I don't know anything about the *%s* stack.  Perhaps it's a typo or perhaps you need to configure it?", project))
		return
	} else {
		stack = &s
	}

	return
}

func getPlaybook(msg *Message, name string, stack *Stack) (playbook *Playbook) {
	if p, ok := stack.Playbooks[name]; !ok {
		msg.Reply("I'm afraid that playbook doesn't exist.")
		return
	} else {
		playbook = &p
	}

	return
}

func listProject(msg *Message, project string, config *Config) {
	stack := getStack(msg, project, config)
	if stack == nil {
		return
	}

	var reply string
	playbooks := stack.GetPlaybookList()
	pc := len(playbooks)
	switch pc {
	case 0:
		reply = fmt.Sprintf("It doesn't look like there are any services associated with *%s*.", project)
	case 1:
		service := playbooks[0]
		pb := stack.Playbooks[service]
		reply = fmt.Sprintf("The *%s* project only has the *%s* service", project, service)
		if pb.About != "" {
			reply += fmt.Sprintf(" designed to %s.", Desentence(pb.About))
		} else {
			reply += " associated with it."
		}

		switch len(pb.Actions) {
		case 0:
		case 1:
			reply += "  This has 1 additional action you can invoke."
		default:
			reply += fmt.Sprintf("  This has %s additional actions you can invoke.", len(pb.Actions))
		}

	default:
		reply = fmt.Sprintf("The *%s* project has %d services associated with it:", project, pc)
		for _, name := range playbooks {
			pb := stack.Playbooks[name]
			reply += fmt.Sprintf("\n  • *%s*", name)
			switch len(pb.Actions) {
			case 0:
			case 1:
				reply += " (with 1 action)"
			default:
				reply += fmt.Sprintf(" (with %d actions)", len(pb.Actions))
			}
			if pb.About != "" {
				reply += fmt.Sprintf(": %s", Desentence(pb.About))
			}
		}
	}

	msg.Reply(reply)
	return
}

func listPlaybook(msg *Message, project, playbook string, config *Config) {
	stack := getStack(msg, project, config)
	if stack == nil {
		return
	}

	pb := getPlaybook(msg, playbook, stack)
	if pb == nil {
		return
	}

	actions := pb.GetActionList()
	var reply string
	ac := len(actions)
	switch ac {
	case 0:
		reply = fmt.Sprintf("There aren't any additional actions associated with *%s*.", playbook)
	case 1:
		name := actions[0]
		action := pb.Actions[name]
		reply = fmt.Sprintf("In addition to `run`, the *%s* playbook has the *%s* action", playbook, name)
		if action.About != "" {
			reply += fmt.Sprintf(" designed to %s.", Desentence(action.About))
		} else {
			reply += " associated with it."
		}
	default:
		reply = fmt.Sprintf("The *%s* playbook has %d actions associated with it:", playbook, ac)
		for _, name := range actions {
			a := pb.Actions[name]
			reply += fmt.Sprintf("\n  • *%s*", name)
			if a.About != "" {
				reply += fmt.Sprintf(": %s", Desentence(a.About))
			}
		}
	}

	msg.Reply(reply)
	return
}

func processList(msg *Message, cmd []string, config *Config) {
	client, err := getDockerClient()
	if err != nil {
		msg.Reply(fmt.Sprintf("I could not create the Docker client: %s", err))
		return
	}

	if _, err = updateDevopsImage(msg, client, config); err != nil {
		return
	}

	if len(config.Stacks) == 0 {
		msg.Reply("I'm sorry; there aren't any projects listed.")
		return
	}

	switch len(cmd) {
	case 0:
		listProjects(msg, config)
	case 1:
		project := cmd[0]
		listProject(msg, project, config)
	case 2:
		project := cmd[0]
		playbook := cmd[1]
		listPlaybook(msg, project, playbook, config)
	default:
		helpList("I'm sorry, I have no idea what you're asking", msg)
	}

	return
}

func updateConfigFromImage(msg Conversation, client *docker.Client, config *Config) (err error) {
	var (
		output    []byte
		exit      int
		lurchYaml string = "lurch.yml"
	)

	exit, output, err = runDockerCommand(client, config.Docker.Image, config.Docker.Tag, []string{"cat", lurchYaml})
	if err != nil {
		msg.Send(fmt.Sprintf("I'm sorry, I couldn't update my configuration from the new image.  The message I got is:\n```%s```", err))
		return
	}

	if exit != 0 {
		result := string(output)
		msg.Send(fmt.Sprintf("I'm sorry, I couldn't update my configuration from the new image.  This is the output I got:\n```%s```", result))
		err = fmt.Errorf("docker command failed: %s", result)
		return
	}

	// Unmarshal the YAML string returned from Docker.
	var stacks map[string]Stack
	if err = yaml.Unmarshal(output, &stacks); err != nil {
		msg.Send(fmt.Sprintf("Oh dear! I couldn't read the %s file from the docker image:\n```%s```", lurchYaml, err))
		return
	}

	// This needs to be wrapped in a mutex.
	config.Stacks = stacks
	return
}

func updateDevopsImage(msg Conversation, client *docker.Client, config *Config) (updated bool, err error) {
	// Check whether the image should even be updated.
	if !config.UpdateImage {
		return
	}

	if updated, err = pullDevopsImage(msg, client, config.Docker.Image, config.Docker.Tag, config.Docker.Auth); err != nil {
		return
	} else if updated {
		// Update the configuration as well.
		if err = updateConfigFromImage(msg, client, config); err != nil {
			return
		}
	}

	return
}

func pullDevopsImage(msg Conversation, client *docker.Client, image, tag string, auth docker.AuthConfiguration) (updated bool, err error) {
	if pulling.IsOn() {
		msg.Send("Try again in a sec: I'm busy pulling the latest devops Docker image.")
		err = errors.New("already pulling image")
		return
	}
	pulling.On()
	defer pulling.Off()

	// Start the timeout.
	timeout := make(chan bool, 1)
	go func() {
		time.Sleep(3 * time.Second)
		timeout <- true
	}()

	status := make(chan string, 1)
	errors := make(chan error, 1)
	go func() {
		if result, err := pullDockerImage(client, image, tag, auth); err != nil {
			errors <- err
		} else {
			status <- result
		}
	}()

	var timeoutSent bool
Loop:
	for {
		select {
		case err = <-errors:
			if timeoutSent {
				msg.Send(fmt.Sprintf("Oops! I've just received this error whilst checking for the image:\n```%s``` You'll need to dig into it I'm afraid :disappointed:.", err.Error()))
			} else {
				msg.Send(fmt.Sprintf("I tried and failed to check for an updated devops Docker image.  This is the message I received:\n```%s``` You'll need to dig into it I'm afraid :disappointed:.", err.Error()))
			}
			break Loop
		case r := <-status:
			if timeoutSent {
				// If a holding message has been sent, the user is entitled to
				// know what the end result is.
				if strings.HasPrefix(r, "Status: Downloaded newer") {
					updated = true
					msg.Send("Great - there's a newer image that I'm now using.")
				} else if strings.HasPrefix(r, "Status: Image is up to date") {
					msg.Send("No new image is available: I'll continue using the existing one...")
				} else {
					msg.Send(fmt.Sprintf("I received this message whilst checking for the image.  Not sure what it means...\n```%s```", r))
				}
			} else if strings.HasPrefix(r, "Status: Downloaded newer") {
				updated = true
				msg.Send("Ah!  I've just retrieved the latest devops Docker image. :triumph:")
			} else if strings.HasPrefix(r, "Status: Image is up to date") {
				// Ignore messages where the image status hasn't changed and no timeout was triggered.
			} else {
				// An unknown message: we'd better pass it on.
				msg.Send(fmt.Sprintf("I'm passing on this message I received when checking for an updated devops Docker image.  Not sure what it means...\n```%s```", r))
			}
			break Loop
		case <-timeout:
			// We're holding things up: update the user with a holding message.
			msg.Send("Just a sec: I'm checking to see if there's an updated devops Docker image...")
			timeoutSent = true
		}
	}

	return
}

func deployAction(msg *Message, config *Config) {
	reply := "You need to specify a project by name.  "
	projects := config.GetStackList()
	if len(projects) > 1 {
		reply += fmt.Sprintf("I'm aware of the following %d projects:\n  • %s", len(projects), strings.Join(projects, "\n  • "))
	} else {
		reply = fmt.Sprintf("The only project I know about is *%s*.", projects[0])
	}
	msg.Reply(reply)
}

func lockProject(msg *Message, project string, state *DeployState) (unlock func()) {
	if !state.Set(project) {
		msg.Reply(fmt.Sprintf("Patience! I'm already busy deploying services from *%s* - please wait until I'm done.", project))
		return
	}

	unlock = func() {
		state.Unset(project)
	}

	return
}

func deployStack(msg *Message, action, stack string, state *DeployState, config *Config) {
	unlock := lockProject(msg, stack, state)
	if unlock == nil {
		return
	}
	defer unlock()

	st := getStack(msg, stack, config)
	if st == nil {
		return
	}

	var reply string
	playbooks := st.GetPlaybookList()
	pc := len(playbooks)

	switch pc {
	case 0:
		reply = fmt.Sprintf("It doesn't look like there are any services associated with *%s*", stack)
	case 1:
		reply = fmt.Sprintf("The *%s* project only has the *%s* service associated with it but you need to explicitly type it.", stack, playbooks[0])
	default:
		reply = fmt.Sprintf("Please specify a service from the *%s* project:\n  • %s", stack, strings.Join(playbooks, "\n  • "))
	}
	msg.Reply(reply)
	return
}

func deployPlaybook(msg *Message, action, stack, playbook string, client *docker.Client, state *DeployState, config *Config) {
	unlock := lockProject(msg, stack, state)
	if unlock == nil {
		return
	}
	defer unlock()

	st := getStack(msg, stack, config)
	if st == nil {
		return
	}

	// Ensure the playbook requested actually exists.
	var pb Playbook
	if p, ok := st.Playbooks[playbook]; !ok {
		msg.Reply(fmt.Sprintf("Hmmm.  I'm not aware of the *%s* service being part of the *%s* project.", playbook, stack))
		return
	} else {
		pb = p
	}

	// Ensure the action is valid.
	var act *Action
	if action != "deploy" {
		if a, ok := pb.Actions[action]; !ok {
			actions := pb.GetActionList()
			// Describe actions that do exist
			switch len(actions) {
			case 0:
				msg.Reply(fmt.Sprintf("I'm afraid the %s service doesn't have any custom actions.", playbook))
			case 1:
				msg.Reply(fmt.Sprintf("Hmmm.  I don't know that action: the only custom action associated with *%s* is *%s*.", stack, playbook))
			case 2:
				msg.Reply(fmt.Sprintf("Hmmm.  I don't know that action: the only custom action associated with *%s* is *%s*.", stack, playbook))
			default:
				msg.Reply(fmt.Sprintf("Hmmm.  I don't know that action: these are the custom actions for *%s* that I'm aware of:\n   • %s", stack, strings.Join(actions, "\n  • ")))
			}

			return
		} else {
			act = &a
		}

		msg.Reply(fmt.Sprintf("OK, I'm running the %s action on the *%s %s* service...", action, stack, playbook))
	} else {
		msg.Reply(fmt.Sprintf("OK, I'm running the *%s %s* service...", stack, playbook))
	}

	// Build the Ansible command.
	args := []string{"ansible-playbook"}
	if act != nil {
		for k, v := range act.Vars {
			args = append(args, []string{"--extra-vars", fmt.Sprintf("%s=%s", k, v)}...)
		}
	}
	args = append(args, pb.Location)

	exit, output, err := runDockerCommand(client, config.Docker.Image, config.Docker.Tag, args)
	if err != nil {
		msg.Reply(fmt.Sprintf("I failed to %s the *%s %s* service: %s", action, stack, playbook, err))
	} else if exit != 0 {
		image := strings.Join([]string{config.Docker.Image, config.Docker.Tag}, ":")
		cmd := fmt.Sprintf("docker pull %s && \\\ndocker run -t --rm %s %s", image, image, strings.Join(args, " "))
		// For some reason Slack doesn't like these two messages concatenated, so send them separately.
		msg.Reply(fmt.Sprintf("I failed to %s the *%s %s* service:\n```%s```", action, stack, playbook, string(output)))
		msg.Reply(fmt.Sprintf("You can replicate this problem from a terminal with:\n```%s```", cmd))
	} else {
		msg.Reply(fmt.Sprintf("*%s %s* successfully %sed.", stack, playbook, action))
	}

	return
}

func processDeploy(msg *Message, cmd []string, state *DeployState, config *Config) {
	client, err := getDockerClient()
	if err != nil {
		msg.Reply(fmt.Sprintf("I could not create the Docker client: %s", err))
		return
	}

	if _, err = updateDevopsImage(msg, client, config); err != nil {
		return
	}

	if len(config.Stacks) == 0 {
		msg.Reply("I'm sorry; there aren't any projects listed.")
		return
	}

	switch len(cmd) {
	case 0:
		msg.Reply("You need to specify at least an action and a project.")
	case 1: // <action>
		deployAction(msg, config)
	case 2: // <action> <stack>
		action, stack := cmd[0], cmd[1]
		deployStack(msg, action, stack, state, config)
	case 3: // <action> <stack> <playbook>
		action, stack, playbook := cmd[0], cmd[1], cmd[2]
		deployPlaybook(msg, action, stack, playbook, client, state, config)
	default: // Unhandled.
		msg.Reply("That sounds way too complicated for a simpleton like me to understand!")
	}

	return
}

func processConnectedEvent(rtm *slack.RTM, channelID string, config *Config) {
	msg := NewChannelMessage(rtm, channelID)

	client, err := getDockerClient()
	if err != nil {
		msg.Send(fmt.Sprintf("I could not create the Docker client: %s", err))
		return
	}

	if updated, err := updateDevopsImage(msg, client, config); err != nil {
		return
	} else if !updated {
		// Perform the initial configuration.
		if err = updateConfigFromImage(msg, client, config); err != nil {
			return
		}
	} else {
		if err = updateConfigFromImage(msg, client, config); err != nil {
			return
		}
	}

	return
}

func processMessage(
	rtm *slack.RTM,
	ev *slack.MessageEvent,
	user *User,
	deployID string,
	state *DeployState,
	config *Config,
	logger *log.Logger) {
	msg := NewMessage(rtm, ev, user)
	if msg == nil {
		return
	}

	// Deal with an empty message.
	if msg.Text == "" {
		msg.Reply("You rang?")
		return
	}

	// Process the command.
	cmd := msg.Command()
	//logger.Printf("Message for Lurch: %s\n", strings.Join(cmd, " "))
	switch cmd[0] {
	case "help":
		processHelp(msg, cmd[1:])

	case "list":
		processList(msg, cmd[1:], config)

	case "deploy":
		fallthrough
	default:
		if deployID != ev.Channel {
			msg.Reply(fmt.Sprintf("I'm sorry, you can only run playbook commands on the *%s* channel. This way everyone is notified.", config.CommandChannel))
		} else {
			processDeploy(msg, cmd, state, config)
		}
	}
}
