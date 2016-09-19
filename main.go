package main

import (
	"errors"
	"fmt"
	"log"
	"net"
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

func catchSignals(exit chan<- bool, rtm *slack.RTM, config *Config, logger *log.Logger) {
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

			// Notify the channels I'm a member of that I'm leaving.
			bc := NewBroadcast(rtm, config.Channels)
			msg := fmt.Sprintf("I have received the %s signal and am leaving. :anguished:", sig.String())
			bc.Send(msg)

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

func UpdateChannels(rtm *slack.RTM, config *Config, logger *log.Logger) (err error) {
	maxAttempts := config.ConnAttempts
	for attempts := 1; attempts <= maxAttempts; attempts++ {
		if err = updateChannels(rtm, config); err == nil {
			break
		}

		// If it's a temporary network error, try again.
		if nerr, ok := err.(net.Error); ok && nerr.Temporary() {
			duration := time.Duration(attempts) * time.Second
			logger.Printf("network error connecting to slack (attempt %d of %d): trying again in %s", attempts, maxAttempts, duration)
			time.Sleep(duration)
			continue
		} else {
			return
		}
	}

	return
}

func run(config *Config, logger *log.Logger) (err error) {
	api := slack.New(config.SlackToken)
	slack.SetLogger(logger)
	api.SetDebug(config.Debug)

	// Obtain a handle on the Slack Real Time Messaging API.
	rtm := api.NewRTM()
	go rtm.ManageConnection()

	// Set the state to check which deployments are ongoing.
	state := NewRunState()

	if err = UpdateChannels(rtm, config, logger); err != nil {
		err = errors.New(fmt.Sprintf("I couldn't set my channel membership: %s", err))
		return
	}

	// Respond to signals.
	exit := make(chan bool, 1)
	go catchSignals(exit, rtm, config, logger)

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
				processConnectedEvent(rtm, config)

			case *slack.DisconnectedEvent:
				var msg string
				if ev.Intentional {
					msg = "sent away"
				} else {
					msg = "forcibly disconnected"
				}
				logger.Printf(msg)

			case *slack.MessageEvent:
				go processMessage(rtm, ev, lurch, state, config, logger)

			case *slack.RTMError:
				logger.Printf("error: %s\n", ev.Error())

			case *slack.InvalidAuthEvent:
				logger.Print("invalid credentials")
				break Loop

			case *slack.GroupLeftEvent:
				config.Channels.RemoveChannel(ev.Channel)
			case *slack.ChannelLeftEvent:
				config.Channels.RemoveChannel(ev.Channel)

			case *slack.GroupJoinedEvent:
				config.Channels.AddChannel(ev.Channel.ID, Group)
			case *slack.ChannelJoinedEvent:
				config.Channels.AddChannel(ev.Channel.ID, Channel)

			default:
				// Ignore other events...
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
			Name:        "disable-pull",
			Usage:       "don't check the registry for newer versions of the docker image",
			EnvVar:      "LURCH_DISABLE_PULL",
			Destination: &config.DisablePull,
		},
		cli.BoolFlag{
			Name:        "enable-dm",
			Usage:       "run playbooks over direct message channels.",
			EnvVar:      "LURCH_ENABLE_DM",
			Destination: &config.EnableDM,
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
		cli.IntFlag{
			Name:        "conn-attempts",
			Usage:       "the maximum number of attempts to be made to connect to Slack on startup",
			EnvVar:      "LURCH_CONN_ATTEMPTS",
			Value:       20,
			Destination: &config.ConnAttempts,
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
