# Lurch - A DevOps Slack Bot

Lurch is a [Slack bot](https://api.slack.com/bot-users) designed to run
[Ansible](https://www.ansible.com/) playbooks. You tell Lurch what to do in a
Slack channel and he will translate those instructions into a command that
invokes an Ansible playbook.  You define the playbooks to do what you want in
terms of deployments.

## Requirements

The Ansible playbooks reside in a Docker image you create.  This image will
include a self-contained Ansible installation.  As such an image such as
[williamyeh/ansible](https://hub.docker.com/r/williamyeh/ansible/) provides a
good base image.  Your image will need to include all required SSH configuration
so that the `ansible-playbook` command can be used successfully to perform
deployments: Lurch will be using this command. The image will also need to
fulfil a few conventions to ensure that Lurch can find the deployments.

The first convention is that there will need to be a top level playbook in the
working directory called `all.yml`.  This should contain Ansible includes for
all playbooks defining individual deployments.  An example `all.yml` would be:

```
---

# Deployments for my-app
- include: deploy/my-app/dev.yml
- include: deploy/my-app/testing.yml
- include: deploy/my-app/production.yml

# Deployments for my-project
- include: deploy/my-project/website.yml
- include: deploy/my-project/wiki.yml
- include: deploy/my-project/forum.yml
```

Assuming the YAML files reference valid playbooks, this top level playbook would
deploy: an application **my-app** with development, testing and production
deployments; a project **my-project** composed of a website, wiki and forum.
Lurch will parse this file and examine each include.

The format of the include path is important: if it matches the pattern
`deploy/<project-name>/<service>.yml` then Lurch will consider it to be a valid
deployment.  In this way multiple services can be associated with an individual
project (or application).

There are no conventions that Lurch expects in relation to the execution of
playbooks with one exception: if asked to restart a service, Lurch will set the
value of the global playbook variable `state` to `restarted`.  It is assumed
that the playbooks will make use of this variable to restart services.

N.B. It's likely that the Ansible image will contain configuration secrets and
you will therefore want keep it in a private Docker Registry rather than on the
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
   0.1.0
   
AUTHOR(S):
   Homme Zwaagstra <hrz@geodata.soton.ac.uk> 
   
COMMANDS:
     help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --slack-token value        your Slack API token [$LURCH_SLACK_TOKEN]
   --docker-image value       the Docker image to manipulate [$LURCH_DOCKER_IMAGE]
   --command-channel value    the channel on which to issue commands to lurch (default: "devops") [$LURCH_COMMAND_CHANNEL]
   --registry-user value      the username for the docker registry [$LURCH_REGISTRY_USER]
   --registry-password value  the password for the docker registry [$LURCH_REGISTRY_PASSWORD]
   --registry-email value     the email for the docker registry [$LURCH_REGISTRY_EMAIL]
   --registry-address value   the server address for the docker registry [$LURCH_REGISTRY_ADDRESS]
   --debug                    produce debugging output [$LURCH_DEBUG]
   --help, -h                 show help
   --version, -v              print the version
```

Key command line options are `--slack-token` and `--docker-image`.  Your unique
Slack token is generated when you add a custom bot integration to your Slack
team.  The Docker image is your custom Ansible deployment image.  If you are
wanting to instruct Lurch on a channel other than the default **devops** channel
you will want to inform Lurch of this using the `--docker-image` option.  The
`--registry-*` options define the connection parameters to a Docker Registry:
this defaults to the Docker Hub which, as described previously, is probably not
what you want for security reasons.  Note that most command line options can
also be specified using environment variables.

Once Lurch is running, he should introduce himself on your command channel.
From there you can interact with him and instruct him to deploy and restart the
playbooks listed in your Ansible docker image.  Type `@lurch help` (or just
`lurch help`) in the command channel to get started.

## Installation

Lurch is designed to be packaged up in a Docker image.  To generate this image
(which will only be a few megabytes in size), do the following:

```
git clone https://github.com/geo-data/lurch.git
cd lurch
make docker
```

This will create an image tagged `geodata/lurch:latest`, which can be run as
follows:

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

## Contributing

Typing `make dev` from project root builds development docker image and runs a
container, placing you at a command prompt within this container.  This uses
Docker Compose, so ensure you have it installed.

The project root is bind mounted to the current working directory in the
container allowing you to edit files on the host and run `make` commands within
the container.  The main command you'll use is `make run` which builds and runs
Lurch using [Realize](https://tockins.github.io/realize/).  This provides live
reloading of the `lurch` binary whenever source files change.

You'll want to prefix `make run` with Lurch specific environment variables
defining your Slack and deployment environment e.g.:

```
LURCH_SLACK_TOKEN=xxx \
LURCH_COMMAND_CHANNEL=deploy \
LURCH_DEBUG=true \
LURCH_DOCKER_IMAGE=my.registry.org/my/devops-image:latest \
LURCH_REGISTRY_ADDRESS=my.registry.org \
LURCH_REGISTRY_EMAIL=me@example.org \
LURCH_REGISTRY_USER=me \
LURCH_REGISTRY_PASSWORD=xxx \
make run
```
