# Lurch - A DevOps Slack Bot

[![GitHub release](https://img.shields.io/github/release/geo-data/lurch.svg)](https://github.com/geo-data/lurch/releases/latest)
[![Travis CI](https://img.shields.io/travis/geo-data/lurch.svg)](https://travis-ci.org/geo-data/lurch)
[![Go Report Card](https://goreportcard.com/badge/github.com/geo-data/lurch)](https://goreportcard.com/report/github.com/geo-data/lurch)
[![GoDoc](https://img.shields.io/badge/documentation-godoc-blue.svg)](https://godoc.org/github.com/geo-data/lurch)

Lurch is a [Slack bot](https://api.slack.com/bot-users) designed to run
[Ansible](https://www.ansible.com/) playbooks. You tell Lurch what to do in a
Slack channel and he will translate those instructions into commands that invoke
Ansible playbooks using the standard `ansible-playbook` utility.

## Requirements

Lurch relies on [Docker](https://www.docker.com/) to run Ansible playbooks: the
Ansible playbooks reside in a Docker image you create.  This decouples your
Ansible configuration from Lurch: all you need is an image containing your
playbooks and which has `ansible-playbook` installed.

How you set up your image is completely up to you, but an image such as
[williamyeh/ansible](https://hub.docker.com/r/williamyeh/ansible/) provides a
good base image.  Your image will need to include all required SSH configuration
so that the `ansible-playbook` command can be used successfully to perform
deployments.  As long as you are able to run your image along the lines of
`docker run my/devops-image:latest ansible-playbook my-playbook.yml` then it
should be usable with Lurch, after adding one more file to the image: the
`lurch.yml` configuration file (see the section *Informing Lurch of your
playbooks*).

It's likely that your Ansible image will contain configuration secrets and you
will therefore want keep it in a private Docker Registry rather than on the
public Docker hub: you can tell Lurch how to access this repository using
command line options detailed in the following section.

## Usage

Once you have created a Docker Ansible image containing your deployments as
described in the previous secion, you will want to invoke Lurch.  Lurch is a
server process that connects to both Slack and Docker.  Output from `./lurch
--help` is as follows:

```
NAME:
   lurch - the Lurch Slack bot.

USAGE:
   lurch [global options] command [command options] [arguments...]
   
VERSION:
   v0.2.0
   
AUTHOR(S):
   Homme Zwaagstra <hrz@geodata.soton.ac.uk> 
   
COMMANDS:
     help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --slack-token value        your Slack API token [$LURCH_SLACK_TOKEN]
   --docker-image value       the docker image containing your Ansible playbooks [$LURCH_DOCKER_IMAGE]
   --disable-pull             don't check the registry for newer versions of the docker image [$LURCH_DISABLE_PULL]
   --enable-dm                run playbooks over direct message channels. [$LURCH_ENABLE_DM]
   --registry-user value      the username for the docker registry [$LURCH_REGISTRY_USER]
   --registry-password value  the password for the docker registry [$LURCH_REGISTRY_PASSWORD]
   --registry-email value     the email for the docker registry [$LURCH_REGISTRY_EMAIL]
   --registry-address value   the server address for the docker registry [$LURCH_REGISTRY_ADDRESS]
   --debug                    produce debugging output [$LURCH_DEBUG]
   --conn-attempts value      the maximum number of attempts to be made to connect to Slack on startup (default: 20) [$LURCH_CONN_ATTEMPTS]
   --help, -h                 show help
   --version, -v              print the version
```

Key command line options are `--slack-token` and `--docker-image`.  Your unique
Slack token is generated when you add a custom bot integration to your Slack
team.  The Docker image is your custom Ansible deployment image.

The `--registry-*` options define the connection parameters to a Docker
Registry: this defaults to the Docker Hub which, as described in the previous
section, is probably not what you want for security reasons.

Note that most command line options can also be specified using environment
variables.

Once Lurch is running, he should introduce himself on your command channel.
From there you can interact with him and instruct him to deploy and restart the
playbooks listed in your Ansible docker image.  Type `@lurch help` (or just
`lurch help`) in the command channel to get started.

## Informing Lurch of your playbooks

Lurch uses a file called `lurch.yml` to define interactions with your Docker
image.  This file should sit in the current working directory of your image.
The following example shows you what a `lurch.yml` might look like:

```
---

docker-registry:
  production:
    playbook: ./deploy/docker-registry/production.yml
    about: Deploy the private Docker Registry

gogs:
  production: 
    playbook: ./deploy/gogs/production.yml
    about: Deploy the Gogs Git serice

my-website:
  production:
    playbook: ./deploy/my-website/production.yml
    about: Deploy the live website
    actions:
      restart:
        about: Restart the website services
        vars:
          state: restarted
      stop:
        about: Stop the website services
        vars:
          state: stopped

  wiki:
    playbook: ./deploy/my-website/wiki.yml
    about: Deploy the wiki

  clean:
    playbook: ./deploy/my-website/clean.yml
    about: Remove all temporary files from the website
```

The file defines, in order of hierarchy: **stacks**; **playbooks**; and
**actions**.  There are three **stacks** in the example: `docker-registry`;
`gogs`; and `my-website`. `docker-registry` has one playbook: `production`. This
contains two standard properties: `playbook` and `about`. `playbook` references
the location of the playbook relative to the current working directory of the
image.  `about` is a sentence describing the effect of running the playbook.

The `gogs` stack is very similar to `docker-registry`.  Things become a little
more interesting in the `my-website` stack.  This stack has three playbooks:
`production`, `wiki` and `clean`.  The `production` playbook contains an
`actions` property in addition to the `playbook` and `about` properties we've
seen before.

`actions` defines alternative actions that can be applied to a playbook by
associating one or more extra playbook variables that are passed to
`ansible-playbook` via its `--extra-vars` command line option.  The `production`
playbook has two actions: `restart`; and `stop` which are described by their
`about` properties.  The `vars` property contains a hash associating variable
names with values.  In each case there's one variable `state` that is used to
alter the action of the playbook.

In this way multiple related playbooks can be grouped together into individual
**stacks**.  Moreover, the default actions of running the playbooks can be
augmented with customised actions.

## Installation

### Via Docker (recommended)

The latest version of Lurch is available from the Docker Registry at
[geodata/lurch:latest](https://hub.docker.com/r/geodata/lurch).  It can be run
as follows:

```
docker run -d -v /var/run/docker.sock:/var/run/docker.sock \
  geodata/lurch:latest \
    --slack-token xxx \
    --docker-image your.registry.com/your/devops-image:version
```

A key point here is that you need to bind mount the docker socket so that Lurch
can communicate with the docker daemon in order to run the devops docker image.

Note that you can set environment variables instead of using Lurch's command
line flags (the `docker run --env-file` flag is a useful option for specifying
environment variables containing secrets like your Slack token).

### Binary download

You can download a self contained `lurch` binary compiled for Linux x86_64 from
the [latest release](https://github.com/geo-data/lurch/releases/latest).

## Contributing

Typing `make dev` from project root builds development docker image and runs a
container, placing you at a command prompt within this container.  This uses
[Docker Compose](https://docs.docker.com/compose/), so ensure you have it
installed.

The project root is bind mounted to the current working directory in the
container allowing you to edit files on the host and run `make` commands within
the container.  The main command you'll use is `make run` which builds and runs
Lurch using [Realize](https://tockins.github.io/realize/).  This provides live
reloading of the `lurch` binary whenever source files change.

You'll want to prefix `make run` with Lurch specific environment variables
defining your Slack and deployment environment e.g.:

```
LURCH_SLACK_TOKEN=xxx \
LURCH_DEBUG=true \
LURCH_DOCKER_IMAGE=my.registry.org/my/devops-image:latest \
LURCH_REGISTRY_ADDRESS=my.registry.org \
LURCH_REGISTRY_EMAIL=me@example.org \
LURCH_REGISTRY_USER=me \
LURCH_REGISTRY_PASSWORD=xxx \
make run
```

## License

[![license](https://img.shields.io/github/license/geo-data/lurch.svg)](https://github.com/geo-data/lurch/blob/master/LICENSE)

MIT - See the file `LICENSE` for details.
