package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/fsouza/go-dockerclient"
	"golang.org/x/net/context"
)

func getDockerClient() (*docker.Client, error) {
	endpoint := "unix:///var/run/docker.sock"
	return docker.NewClient(endpoint)
}

func pullDockerImage(client *docker.Client, image, tag string, auth docker.AuthConfiguration) (result string, err error) {
	var wg sync.WaitGroup
	wg.Add(2) // Wait for the puller and collector goroutines.

	// A pipe connecting the puller and the collector.
	r, w := io.Pipe()

	// Pull the image.
	go func() {
		defer wg.Done()
		defer w.Close()
		err = client.PullImage(
			docker.PullImageOptions{
				Repository:        image,
				Tag:               tag,
				OutputStream:      w,
				RawJSONStream:     false,
				InactivityTimeout: 30 * time.Second,
				Context:           context.Background(),
			},
			auth,
		)
	}()

	// Collect the output.
	go func() {
		defer wg.Done()
		defer r.Close()
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			result = scanner.Text()
		}
		if err == nil {
			err = scanner.Err()
		}
	}()

	// Wait for the puller and collector to finish.
	wg.Wait()
	return
}

func runDockerCommand(client *docker.Client, image, tag string, args, env []string) (exit int, output []byte, err error) {
	// Set the timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var fullImage string = image
	if tag != "" {
		fullImage += fmt.Sprintf(":%s", tag)
	}

	// Create the container.
	var cont *docker.Container
	if cont, err = client.CreateContainer(docker.CreateContainerOptions{
		"",
		&docker.Config{
			Image: fullImage,
			Cmd:   args,
			Env:   env,
		},
		&docker.HostConfig{
			AutoRemove: true,
		},
		nil,
		ctx,
	}); err != nil {
		return
	}

	// Capture all output from the container. This blocks so run it in parallel
	// to enable us to start the container.
	var buf bytes.Buffer
	go func() {
		if err = client.AttachToContainer(docker.AttachToContainerOptions{
			Container:    cont.ID,
			OutputStream: &buf,
			Logs:         true,
			Stdout:       true,
			Stderr:       true,
			Stream:       true,
		}); err != nil {
			return
		}
	}()

	// Start the container and begin capturing the output.
	if err = client.StartContainer(cont.ID, nil); err != nil {
		return
	}

	// Wait for the container to exit, by which time all output should be complete.
	if exit, err = client.WaitContainer(cont.ID); err != nil {
		return
	}

	output = buf.Bytes()
	return
}
