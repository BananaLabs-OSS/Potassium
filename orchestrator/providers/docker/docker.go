package docker

import (
	"context"
	"fmt"
	"strconv"

	"github.com/bananalabs-oss/potassium/orchestrator"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
)
import "github.com/docker/docker/client"

type DockerProvider struct {
	client *client.Client
}

func New() (*DockerProvider, error) {
	// try create client
	cli, err := client.NewClientWithOpts(client.FromEnv)

	// if client creation fails
	if err != nil {
		return nil, err
	}

	// Return provider
	return &DockerProvider{
		client: cli,
	}, nil
}

// List - takes filter, returns slice
func (d *DockerProvider) List(ctx context.Context, filter map[string]string) ([]orchestrator.Server, error) {
	// Request docker container list
	c, err := d.client.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		return nil, err
	}

	// Loop through list of containers
	servers := []orchestrator.Server{}
	for _, c := range c {
		// Convert status
		var status orchestrator.ServerStatus
		switch c.State {
		case "running":
			status = orchestrator.StatusRunning
		case "exited":
			status = orchestrator.StatusStopped
		default:
			status = orchestrator.StatusError
		}

		// Get IP
		var ip string
		for _, network := range c.NetworkSettings.Networks {
			ip = network.IPAddress
			break
		}

		// Convert ports
		ports := map[string]int{}
		for _, p := range c.Ports {
			if p.PublicPort != 0 {
				ports[fmt.Sprintf("%d", p.PrivatePort)] = int(p.PublicPort)
			}
		}

		server := orchestrator.Server{
			ID:     c.ID,
			Name:   c.Names[0],
			Status: status,
			IP:     ip,
			Ports:  ports,
		}

		servers = append(servers, server)
	}

	return servers, nil
}

// Get - takes id, returns pointer
func (d *DockerProvider) Get(ctx context.Context, id string) (*orchestrator.Server, error) {
	// Get container by ID
	c, err := d.client.ContainerInspect(ctx, id)
	if err != nil {
		return nil, err
	}

	// Convert status
	var status orchestrator.ServerStatus
	switch c.State.Status {
	case "running":
		status = orchestrator.StatusRunning
	case "exited":
		status = orchestrator.StatusStopped
	default:
		status = orchestrator.StatusError
	}

	// Get IP
	var ip string
	for _, network := range c.NetworkSettings.Networks {
		ip = network.IPAddress
		break
	}

	// Convert ports
	ports := map[string]int{}
	for port, bindings := range c.NetworkSettings.Ports {
		if len(bindings) > 0 {
			hostPort, err := strconv.Atoi(bindings[0].HostPort)
			if err == nil {
				ports[port.Port()] = hostPort
			}
		}
	}

	server := orchestrator.Server{
		ID:     c.ID,
		Name:   c.Name,
		Status: status,
		IP:     ip,
		Ports:  ports,
	}

	return &server, nil
}

// Allocate - takes request, returns pointer
func (d *DockerProvider) Allocate(ctx context.Context, req orchestrator.AllocateRequest) (*orchestrator.Server, error) {
	// Build env slice
	env := []string{}
	for key, value := range req.Environment {
		env = append(env, key+"="+value)
	}

	// Build buinds from request.volumes
	var binds []string
	for host, c := range req.Volumes {
		binds = append(binds, host+":"+c)
	}

	// Build exposed ports (container side)
	exposedPorts := nat.PortSet{}
	portBindings := nat.PortMap{}

	for _, p := range req.Ports {
		containerPort := nat.Port(fmt.Sprintf("%d/%s", p.Container, p.Protocol))
		exposedPorts[containerPort] = struct{}{}
		portBindings[containerPort] = []nat.PortBinding{
			{
				HostIP:   "",
				HostPort: fmt.Sprintf("%d", p.Host),
			},
		}
	}

	// Create container
	resp, err := d.client.ContainerCreate(
		ctx,
		&container.Config{
			Image:        req.Image,
			Env:          env, // Env Slice
			ExposedPorts: exposedPorts,
		},
		&container.HostConfig{
			Binds:        binds,
			PortBindings: portBindings,
		},
		nil, // Network Config
		nil, // Platform
		"",  // Name (Docker generates one)
	)
	if err != nil {
		return nil, err
	}

	// Start container
	err = d.client.ContainerStart(ctx, resp.ID, container.StartOptions{})
	if err != nil {
		return nil, err
	}

	// Get and return container as server
	return d.Get(ctx, resp.ID)
}

// Deallocate - takes id, returns error
func (d *DockerProvider) Deallocate(ctx context.Context, id string) error {
	// Stop container
	err := d.client.ContainerStop(ctx, id, container.StopOptions{})
	if err != nil {
		return err
	}

	// Remove container
	err = d.client.ContainerRemove(ctx, id, container.RemoveOptions{})
	if err != nil {
		return err
	}

	return nil
}
