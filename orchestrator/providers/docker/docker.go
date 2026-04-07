package docker

import (
	"bytes"
	"context"
	"fmt"
	"strconv"

	"github.com/bananalabs-oss/potassium/orchestrator"
	"github.com/docker/docker/api/types/blkiodev"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-connections/nat"
)

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

		// Inspect to get HostConfig limits (used by reconcile)
		if inspect, err := d.client.ContainerInspect(ctx, c.ID); err == nil {
			if inspect.HostConfig != nil {
				server.MemoryLimit = inspect.HostConfig.Memory
				if inspect.HostConfig.NanoCPUs > 0 {
					server.CPULimit = float64(inspect.HostConfig.NanoCPUs) / 1e9
				}
			}
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
	if c.HostConfig != nil {
		server.MemoryLimit = c.HostConfig.Memory
		if c.HostConfig.NanoCPUs > 0 {
			server.CPULimit = float64(c.HostConfig.NanoCPUs) / 1e9
		}
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

	// Build binds from request.volumes
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

	// Only bind to host if not using overlay network
	if req.Network != "" {
		portBindings = nat.PortMap{}
	}

	// Build network config if specified
	var networkConfig *network.NetworkingConfig
	if req.Network != "" {
		endpointConfig := &network.EndpointSettings{}
		if req.IP != "" {
			endpointConfig.IPAMConfig = &network.EndpointIPAMConfig{
				IPv4Address: req.IP,
			}
		}
		networkConfig = &network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				req.Network: endpointConfig,
			},
		}
	}

	// Build resource limits
	resources := container.Resources{}
	if req.MemoryLimit > 0 {
		resources.Memory = req.MemoryLimit
	}
	if req.CPULimit > 0 {
		resources.NanoCPUs = int64(req.CPULimit * 1e9)
	}
	if req.PidsLimit > 0 {
		resources.PidsLimit = &req.PidsLimit
	}
	if req.MemorySwap != 0 {
		resources.MemorySwap = req.MemorySwap
	}
	// Disk I/O rate limits (applied to all block devices)
	if req.DiskIOReadBps > 0 {
		resources.BlkioDeviceReadBps = []*blkiodev.ThrottleDevice{
			{Path: "/dev/sda", Rate: uint64(req.DiskIOReadBps)},
			{Path: "/dev/nvme0n1", Rate: uint64(req.DiskIOReadBps)},
		}
	}
	if req.DiskIOWriteBps > 0 {
		resources.BlkioDeviceWriteBps = []*blkiodev.ThrottleDevice{
			{Path: "/dev/sda", Rate: uint64(req.DiskIOWriteBps)},
			{Path: "/dev/nvme0n1", Rate: uint64(req.DiskIOWriteBps)},
		}
	}

	// Disk size limit (requires overlay2 + xfs with pquota)
	storageOpt := map[string]string{}
	if req.DiskSizeLimit > 0 {
		storageOpt["size"] = fmt.Sprintf("%d", req.DiskSizeLimit)
	}

	// Create container
	resp, err := d.client.ContainerCreate(
		ctx,
		&container.Config{
			Image:        req.Image,
			Env:          env, // Env Slice
			ExposedPorts: exposedPorts,
			OpenStdin:    true,
		},
		&container.HostConfig{
			Binds:        binds,
			PortBindings: portBindings,
			Resources:    resources,
			StorageOpt:   storageOpt,
		},
		networkConfig, // Network Config
		nil,           // Platform
		req.Name,      // Name (empty → Docker generates one)
	)
	if err != nil {
		return nil, err
	}

	// Start container
	err = d.client.ContainerStart(ctx, resp.ID, container.StartOptions{})
	if err != nil {
		// Clean up the created container on start failure
		d.client.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		return nil, err
	}

	// Get and return container as server
	return d.Get(ctx, resp.ID)
}

// Restart - stops and restarts a container (works whether running or stopped).
func (d *DockerProvider) Restart(ctx context.Context, id string) error {
	return d.client.ContainerRestart(ctx, id, container.StopOptions{})
}

// Exec runs a command inside a container and returns stdout.
func (d *DockerProvider) Exec(ctx context.Context, id string, cmd []string) (string, error) {
	execID, err := d.client.ContainerExecCreate(ctx, id, container.ExecOptions{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		return "", fmt.Errorf("exec create failed: %w", err)
	}

	resp, err := d.client.ContainerExecAttach(ctx, execID.ID, container.ExecAttachOptions{})
	if err != nil {
		return "", fmt.Errorf("exec attach failed: %w", err)
	}
	defer resp.Close()

	var stdout, stderr bytes.Buffer
	if _, err := stdcopy.StdCopy(&stdout, &stderr, resp.Reader); err != nil {
		return "", fmt.Errorf("exec read failed: %w", err)
	}

	inspect, err := d.client.ContainerExecInspect(ctx, execID.ID)
	if err != nil {
		return "", fmt.Errorf("exec inspect failed: %w", err)
	}

	if inspect.ExitCode != 0 {
		return "", fmt.Errorf("exec exit code %d: %s", inspect.ExitCode, stderr.String())
	}

	return stdout.String(), nil
}

// Logs returns the last `tail` lines of stdout+stderr from a container.
func (d *DockerProvider) Logs(ctx context.Context, id string, tail int) (string, error) {
	opts := container.LogsOptions{
		Tail:       strconv.Itoa(tail),
		ShowStdout: true,
		ShowStderr: true,
	}
	reader, err := d.client.ContainerLogs(ctx, id, opts)
	if err != nil {
		return "", fmt.Errorf("container logs failed: %w", err)
	}
	defer reader.Close()

	var buf bytes.Buffer
	if _, err := stdcopy.StdCopy(&buf, &buf, reader); err != nil {
		return "", fmt.Errorf("reading logs failed: %w", err)
	}

	return buf.String(), nil
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
