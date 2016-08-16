##
# Docker image for developing the Web Observatory Geo API.
#
# This provides a development environment for the application.  See the root
# `Dockerfile` for an image suitable for production deployment.
#

FROM golang:1.7

MAINTAINER Homme Zwaagstra <hrz@geodata.soton.ac.uk>

COPY ./ /go/src/geodata/lurch/

WORKDIR /go/src/geodata/lurch/

RUN ./docker/build.sh

EXPOSE 8080

CMD ["./lurch"]
