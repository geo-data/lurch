package main

import (
	"fmt"
	"log"
	"sort"
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
• *%s* - restart an application.
• *%s* - list applications I can deploy.
Use *%s* for further details.`,
		intro,
		"`deploy`",
		"`restart`",
		"`list`",
		"`help <command>`",
	))
}

func processHelp(msg *Message, cmd []string) {
	if len(cmd) == 0 {
		sendHelp("Hmmm", msg)
		return
	}

	switch cmd[0] {
	case "deploy":
		msg.Reply("Use *`deploy <project> <service>...`* to deploy one or more services related to a project. If you specified more than one service they will be deployed sequentially.")
	case "restart":
		msg.Reply("Use *`restart <project> <service>...`* to re-deploy one or more services related to a project.  If you specified more than one service they will be restarted sequentially.")
	case "list":
		msg.Reply("Use *`list`* to find the projects I can deal with, and *`list <project>...`* to find services related to one or more projects.")
	default:
		sendHelp("I am a simple being: I don't understand", msg)
	}
}

type ProjectList map[string][]string

func getProjectList(client *docker.Client, image, tag string) (list ProjectList, err error) {
	var (
		output []byte
		exit   int
	)
	exit, output, err = runDockerCommand(client, image, tag, []string{"cat", "all.yml"})
	if err != nil {
		return
	}

	if exit != 0 {
		err = fmt.Errorf("docker command failed: %s", string(output))
		return
	}

	// Unmarshal the YAML string returned from Docker.
	var includes []struct {
		Include string `yaml:"include"`
	}

	if err = yaml.Unmarshal(output, &includes); err != nil {
		return
	}

	list = make(ProjectList)
	for _, include := range includes {
		parts := strings.Split(include.Include, "/")
		if len(parts) != 3 && parts[0] != "deploy" {
			continue
		}
		project, service := parts[1], parts[2]

		// Strip of the extension if applicable.
		if idx := strings.LastIndex(service, "."); idx != -1 {
			service = service[:idx]
		}

		list[project] = append(list[project], service)
	}

	return
}

// getProjects returns an ordered list of projects from list.
func getProjects(list ProjectList) (projects []string) {
	for project, _ := range list {
		projects = append(projects, project)
	}
	sort.Strings(projects)
	return
}

func processList(msg *Message, cmd []string, config *Config) {
	client, err := getDockerClient()
	if err != nil {
		msg.Reply(fmt.Sprintf("I could not create the Docker client: %s", err))
		return
	}

	if pullDevopsImage(msg, client, config.Docker.Image, config.Docker.Tag, config.Docker.Auth) != nil {
		return
	}

	list, err := getProjectList(client, config.Docker.Image, config.Docker.Tag)
	if err != nil {
		msg.Reply(fmt.Sprintf("I could not get the project list: %s", err))
		return
	}

	if len(list) == 0 {
		msg.Reply("I'm sorry; there aren't any projects listed.")
		return
	}

	projects := getProjects(list)

	if len(cmd) == 0 {
		var reply string
		if len(projects) > 1 {
			reply = fmt.Sprintf("I know about the following %d projects:\n  • %s", len(projects), strings.Join(projects, "\n  • "))
		} else {
			reply = fmt.Sprintf("I only know about the *%s* project.", projects[0])
		}
		msg.Reply(reply)
		return
	}

	var unknown []string
	for _, project := range cmd {
		if _, ok := list[project]; !ok {
			unknown = append(unknown, project)
		}
	}

	unknownCount := len(unknown)
	if unknownCount == len(cmd) {
		switch len(unknown) {
		case 0:
		case 1:
			msg.Reply("I'm afraid the project doesn't exist.")
			return
		default:
			msg.Reply("I'm afraid the projects you asked for don't exist.")
			return
		}
	} else if unknownCount == 1 {
		msg.Reply(fmt.Sprintf("I'm afraid the project *%s* doesn't exist.", unknown[0]))
		return
	} else if unknownCount > 1 {
		msg.Reply(fmt.Sprintf("I'm afraid these projects don't exist:\n  • " + strings.Join(unknown, "\n  • ")))
		return
	}

	var reply string
	if len(cmd) == 1 {
		serviceCount := len(list[cmd[0]])
		if serviceCount == 1 {
			reply = fmt.Sprintf("The *%s* project only has the *%s* service associated with it.", cmd[0], list[cmd[0]][0])
		} else {
			reply = fmt.Sprintf("The *%s* project has %d services associated with it:\n  • %s", cmd[0], serviceCount, strings.Join(list[cmd[0]], "\n  • "))
		}
	} else {
		reply = "I can help with the following services as associated with their projects:"
		for _, project := range cmd {
			reply += fmt.Sprintf("\n  • *%s*\n        • %s", project, strings.Join(list[project], "\n        • "))
		}
	}

	msg.Reply(reply)
	return
}

func pullDevopsImage(msg *Message, client *docker.Client, image, tag string, auth docker.AuthConfiguration) (err error) {
	if pulling.IsOn() {
		msg.Reply("Try again in a sec: I'm busy pulling the latest devops Docker image.")
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
	go func() {
		var result string
		if result, err = pullDockerImage(client, image, tag, auth); err != nil {
			status <- err.Error()
		} else {
			status <- result
		}
	}()

	var timeoutSent bool
Loop:
	for {
		select {
		case r := <-status:
			if timeoutSent {
				// If a holding message has been sent, the user is entitled to
				// know what the end result is.
				msg.Reply(r + ".")
			} else if strings.HasPrefix(r, "Error:") {
				msg.Reply(fmt.Sprintf("I tried and failed to check for an updated devops Docker image: %s. :disappointed:", r))
			} else if strings.HasPrefix(r, "Status: Downloaded newer") {
				msg.Reply("Ah!  I've just retrieved the latest devops Docker image. :triumph:")
			} else {
				// Ignore messages where the image status hasn't changed.
			}
			break Loop
		case <-timeout:
			// We're holding things up: update the user with a holding message.
			msg.Reply("Just a sec: I'm checking to see if there's an updated devops Docker image...")
			timeoutSent = true
		}
	}

	return
}

func processDeploy(msg *Message, cmd []string, restart bool, state *DeployState, config *Config) {
	client, err := getDockerClient()
	if err != nil {
		msg.Reply(fmt.Sprintf("I could not create the Docker client: %s", err))
		return
	}

	if pullDevopsImage(msg, client, config.Docker.Image, config.Docker.Tag, config.Docker.Auth) != nil {
		return
	}

	list, err := getProjectList(client, config.Docker.Image, config.Docker.Tag)
	if err != nil {
		msg.Reply(fmt.Sprintf("I could not get the project list: %s", err))
		return
	}

	if len(list) == 0 {
		msg.Reply("I'm sorry; there aren't any projects listed.")
		return
	}

	if len(cmd) == 0 {
		reply := "You need to deploy a project by name.  "
		projects := getProjects(list)
		if len(projects) > 1 {
			reply += fmt.Sprintf("I'm aware of the following %d projects:\n  • %s", len(projects), strings.Join(projects, "\n  • "))
		} else {
			reply = fmt.Sprintf("The only project I know about is *%s*.", projects[0])
		}
		msg.Reply(reply)
		return
	}

	project := cmd[0]
	if !state.Set(project) {
		msg.Reply(fmt.Sprintf("Patience! I'm already busy deploying services from *%s* - please wait until I'm done.", project))
		return
	}
	defer state.Unset(project)

	existingServices, ok := list[project]
	if !ok {
		msg.Reply(fmt.Sprintf("Oh dear.  I'm afraid I don't know anything about the *%s* project.  Perhaps it's a typo or perhaps you need to configure the project for deployment?", project))
		return
	}

	if len(cmd) == 1 {
		var reply string
		serviceCount := len(existingServices)
		if serviceCount == 1 {
			reply = fmt.Sprintf("The *%s* project only has the *%s* service associated with it but you need to explicitly deploy it.", project, existingServices[0])
		} else {
			reply = fmt.Sprintf("Please specify one or more services from the *%s* project to deploy:\n  • %s", project, strings.Join(existingServices, "\n  • "))
		}
		msg.Reply(reply)
		return
	}

	// Ensure the services requested actually exist.
	services := cmd[1:]
	var badServices []string
Services:
	for _, service := range services {
		for _, existing := range existingServices {
			if service == existing {
				continue Services
			}
		}
		badServices = append(badServices, service)
	}

	// If any services specified don't exist, alert the caller.
	switch len(badServices) {
	case 0:
		// Skip.
	case 1:
		msg.Reply(fmt.Sprintf("Hmmm.  I'm not aware of the *%s* service being part of the *%s* project.", badServices[0], project))
		return
	case 2:
		msg.Reply(fmt.Sprintf("Hmmm.  Neither the *%s* nor the *%s* services appear to be part of the *%s* project.", badServices[0], badServices[1], project))
		return
	default:
		msg.Reply(fmt.Sprintf("Hmmm.  None of the following services appear to be part of the *%s* project:\n  • %s", project, strings.Join(badServices, "\n  • ")))
		return
	}

	// Deploy the services.
	var action string
	if restart {
		action = "restart"
	} else {
		action = "deploy"
	}
	for _, service := range services {
		msg.Reply(fmt.Sprintf("I'm %sing the *%s %s* service...", action, project, service))

		args := []string{"ansible-playbook"}
		if restart {
			args = append(args, []string{"--extra-vars", "state=restarted"}...)
		}

		playbook := strings.Join([]string{"deploy", project, fmt.Sprintf("%s.yml", service)}, "/")
		args = append(args, playbook)

		exit, output, err := runDockerCommand(client, config.Docker.Image, config.Docker.Tag, args)
		if err != nil {
			msg.Reply(fmt.Sprintf("I failed to %s the *%s %s* service: %s", action, project, service, err))
		} else if exit != 0 {
			cmd := fmt.Sprintf("docker pull dallas.geodata.soton.ac.uk:6040/geodata/devops:latest && \\\ndocker run -t --rm dallas.geodata.soton.ac.uk:6040/geodata/devops:latest %s", strings.Join(args, " "))
			// For some reason Slack doesn't like these two messages concatenated, so send them separately.
			msg.Reply(fmt.Sprintf("I failed to %s the *%s %s* service:\n```%s```", action, project, service, string(output)))
			msg.Reply(fmt.Sprintf("You can replicate this problem from a terminal with:\n```%s```", cmd))
		} else {
			msg.Reply(fmt.Sprintf("*%s %s* successfully %sed.", project, service, action))
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
	var restart bool
	switch cmd[0] {
	case "help":
		processHelp(msg, cmd[1:])

	case "list":
		processList(msg, cmd[1:], config)

	case "restart":
		restart = true
		fallthrough
	case "deploy":
		if deployID != ev.Channel {
			msg.Reply(fmt.Sprintf("I'm sorry, you can only `%s` on the *%s* channel. This way everyone is notified.", cmd[0], config.CommandChannel))
		}
		processDeploy(msg, cmd[1:], restart, state, config)

	default:
		sendHelp("I don't understand", msg)
	}
}
