##
# Makefile for the Lurch Slack bot.
#
# Targets:
# - clean     delete all generated files.
# - run       run the service.
# - build     compile executable.
#
# Meta targets:
# - all is the default target; it builds the lurch binary.
#

# Golang source files.
SRC_FILES := $(shell ls *.go)

# Build dependencies.
BUILD_DEPS := vendor $(SRC_FILES)

# Create production Docker image components by default.
all: build

# Create a development environment.
dev:
	docker-compose build --force-rm lurch && \
	docker-compose run lurch bash

# Create components required for the production Docker image.
build: lurch cacert.pem

# Run the tests.
test:
	drone exec

# Remove automatically generated files.
clean:
	@rm -f lurch
	@rm -rf vendor
	@rm -f cacert.pem

# Run the service.
run: $(BUILD_DEPS)
	go run *.go

# Build an executable optimised for a linux container environment. See
# <https://medium.com/@kelseyhightower/optimizing-docker-images-for-static-binaries-b5696e26eb07#.otbjvqo3i>.
lurch: $(BUILD_DEPS)
	CGO_ENABLED=0 GOOS=linux go build -a -tags netgo -ldflags '-w' -o lurch

vendor: glide.yaml glide.lock
	glide install && \
	touch -c vendor

glide.lock:
	glide update && \
	touch -c vendor

glide.yaml:
	glide init

# Get the PEM root certificates for use in the production docker image.
cacert.pem:
	curl --silent --location https://curl.haxx.se/ca/cacert.pem > cacert.pem

# Targets without filesystem equivalents.
.PHONY: all build clean run dev
