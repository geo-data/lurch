package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fsouza/go-dockerclient"
	"github.com/nlopes/slack"
	reaper "github.com/ramr/go-reaper"
	"github.com/urfave/cli"
)

var version, commit string

func catchSignals(exit chan<- bool, rtm *slack.RTM, channelID string, logger *log.Logger) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		for sig := range c {
			exit <- true // Notify the application we're exiting.

			logger.Printf(fmt.Sprintf("received %s signal", sig.String()))

			// Start the timeout.
			timeout := make(chan bool, 1)
			go func() {
				time.Sleep(5 * time.Second)
				timeout <- true
			}()

			// Notify the channel that I'm leaving.
			msg := fmt.Sprintf("I have received the %s signal and am leaving. :anguished:", sig.String())
			rtm.SendMessage(rtm.NewOutgoingMessage(msg, channelID))

			// Wait for the received response or timeout.
			for {
				select {
				case evt := <-rtm.IncomingEvents:
					switch ev := evt.Data.(type) {
					case *slack.AckMessage:
						if ev.Text == msg { // The message has been round tripped.
							logger.Fatalf("exiting after notifying channel of %s signal", sig.String())
						}
					}
				case <-timeout:
					logger.Fatalf("failed to notify channel about %s signal before timeout", sig.String())
				}
			}
		}
	}()
}

func run(config *Config, logger *log.Logger) (err error) {
	api := slack.New(config.SlackToken)
	slack.SetLogger(logger)
	api.SetDebug(config.Debug)

	var channelID string
	channelID, err = GetIDFromName(api, config.CommandChannel)
	if err != nil {
		return
	} else if channelID == "" {
		err = fmt.Errorf("the command channel does not exist: %s", config.CommandChannel)
		return
	}

	// Obtain a handle on the Slack Real Time Messaging API.
	rtm := api.NewRTM()
	go rtm.ManageConnection()

	// Set the state to check which deployments are ongoing.
	state := NewDeployState()

	// Respond to signals.
	exit := make(chan bool, 1)
	go catchSignals(exit, rtm, channelID, logger)

	// The main event loop.
	var lurch *User
Loop:
	for {
		select {
		case msg := <-rtm.IncomingEvents:
			switch ev := msg.Data.(type) {
			case *slack.HelloEvent:
				// Ignore hello

			case *slack.ConnectedEvent:
				//fmt.Println("Infos:", ev.Info)
				//fmt.Println("Connection counter:", ev.ConnectionCount)
				lurch = NewUser(ev.Info.User)
				go processConnectedEvent(rtm, channelID, config)

			case *slack.DisconnectedEvent:
				var msg string
				if ev.Intentional {
					msg = fmt.Sprintf("sent away from channel %s", config.CommandChannel)
				} else {
					msg = fmt.Sprintf("forcibly disconnected from channel %s", config.CommandChannel)
				}
				logger.Printf(msg)

			case *slack.MessageEvent:
				//fmt.Printf("Message : %v\n", ev)
				go processMessage(rtm, ev, lurch, channelID, state, config, logger)

			case *slack.RTMError:
				logger.Printf("error: %s\n", ev.Error())

			case *slack.InvalidAuthEvent:
				logger.Print("invalid credentials")
				break Loop

			default:
				// Ignore other events..
				//fmt.Printf("Unexpected: %v\n", msg.Data)
			}
		case <-exit:
			// The application is exiting.
			break Loop
		}
	}

	// Wait for goroutines to finish, specifically the signal handler.
	select {}
}

func init() {
	cli.VersionPrinter = func(c *cli.Context) {
		fmt.Printf("%s commit=%s\n", c.App.Version, commit)
	}
}

func main() {
	//  Start background reaping of orphaned child processes.
	go reaper.Reap()

	var config Config
	const name string = "lurch"

	app := cli.NewApp()
	app.Name, config.BotName = name, name
	app.Version = version

	app.Authors = []cli.Author{
		{
			Name:  "Homme Zwaagstra",
			Email: "hrz@geodata.soton.ac.uk",
		},
	}
	app.Usage = "the Lurch Slack bot."

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:        "slack-token",
			Usage:       "your Slack API token",
			EnvVar:      "LURCH_SLACK_TOKEN",
			Destination: &config.SlackToken,
		},
		cli.StringFlag{
			Name:   "docker-image",
			Usage:  "the docker image containing your Ansible playbooks",
			EnvVar: "LURCH_DOCKER_IMAGE",
		},
		cli.BoolFlag{
			Name:        "update-image",
			Usage:       "check the registry for newer versions of the docker image",
			EnvVar:      "LURCH_UPDATE_IMAGE",
			Destination: &config.UpdateImage,
		},
		cli.StringFlag{
			Name:        "command-channel",
			Usage:       "the channel on which to issue commands to lurch",
			Value:       "devops",
			EnvVar:      "LURCH_COMMAND_CHANNEL",
			Destination: &config.CommandChannel,
		},
		cli.StringFlag{
			Name:        "registry-user",
			Usage:       "the username for the docker registry",
			EnvVar:      "LURCH_REGISTRY_USER",
			Destination: &config.Docker.Auth.Username,
		},
		cli.StringFlag{
			Name:        "registry-password",
			Usage:       "the password for the docker registry",
			EnvVar:      "LURCH_REGISTRY_PASSWORD",
			Destination: &config.Docker.Auth.Password,
		},
		cli.StringFlag{
			Name:        "registry-email",
			Usage:       "the email for the docker registry",
			EnvVar:      "LURCH_REGISTRY_EMAIL",
			Destination: &config.Docker.Auth.Email,
		},
		cli.StringFlag{
			Name:        "registry-address",
			Usage:       "the server address for the docker registry",
			EnvVar:      "LURCH_REGISTRY_ADDRESS",
			Destination: &config.Docker.Auth.ServerAddress,
		},
		cli.BoolFlag{
			Name:        "debug",
			Usage:       "produce debugging output",
			EnvVar:      "LURCH_DEBUG",
			Destination: &config.Debug,
		},
	}

	app.Action = func(c *cli.Context) (err error) {
		logger := log.New(os.Stdout, fmt.Sprintf("%s: ", config.BotName), log.Lshortfile|log.LstdFlags)

		if config.SlackToken == "" {
			err = errors.New("no slack token is provided")
			logger.Println(err)
			return
		}

		if image := c.String("docker-image"); image == "" {
			err = errors.New("no docker image is provided")
			logger.Println(err)
			return
		} else {
			config.Docker.Image, config.Docker.Tag = docker.ParseRepositoryTag(image)
		}

		if err = run(&config, logger); err != nil {
			logger.Println(err)
		}

		return
	}
	app.Run(os.Args)
}
