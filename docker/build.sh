#!/bin/bash

##
# This script creates the docker development environment.
#
# This should be run in a clean base image.
#

# Exit on any error.
set -e

# Install glide.
curl -LsS https://github.com/Masterminds/glide/releases/download/0.10.2/glide-0.10.2-linux-amd64.tar.gz \
		| tar xzO linux-amd64/glide > /usr/local/bin/glide
chmod +x /usr/local/bin/glide

# Install rsync (used in the Drone testing).
apt-get update -y
apt-get install -y rsync

# Change to the source root directory.
SRC_DIR="$(dirname $( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd ) )"
cd $SRC_DIR

# Get the golang dependencies.
make clean vendor

# Install realize.
cd ${SRC_DIR}/vendor/github.com/tockins/realize
go install

# Build lurch.
cd $SRC_DIR
make build

# Clean up.
apt-get clean
rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*
