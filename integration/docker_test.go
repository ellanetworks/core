package integration_test

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/netip"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
)

type DockerClient struct {
	*client.Client
}

func NewDockerClient() (*DockerClient, error) {
	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("create docker client: %w", err)
	}

	return &DockerClient{Client: cli}, nil
}

func (dc *DockerClient) CreateNetwork(ctx context.Context, name string, subnet netip.Prefix) error {
	createOpts := client.NetworkCreateOptions{
		Driver: "bridge",
		IPAM: &network.IPAM{
			Driver: "default",
			Config: []network.IPAMConfig{
				{
					Subnet: subnet,
				},
			},
		},
	}

	_, err := dc.NetworkCreate(ctx, name, createOpts)
	if err != nil {
		return fmt.Errorf("failed to create network %s: %w", name, err)
	}

	return nil
}

func (dc *DockerClient) CreateEllaCoreContainerWithConfig(ctx context.Context, configPath string) error {
	absCfg, err := filepath.Abs(configPath)
	if err != nil {
		return fmt.Errorf("resolve config path: %w", err)
	}

	port, err := network.ParsePort("5002/tcp")
	if err != nil {
		return fmt.Errorf("parse port: %w", err)
	}

	cfg := &container.Config{
		Image: "ella-core:latest",
		Cmd:   []string{"exec", "/bin/core", "--config", "/core.yaml"},
		ExposedPorts: network.PortSet{
			port: struct{}{},
		},
	}
	hostIP := netip.MustParseAddr("0.0.0.0")
	hostCfg := &container.HostConfig{
		Privileged:  true,
		Binds:       []string{absCfg + ":/core.yaml:ro", "/sys/fs/bpf:/sys/fs/bpf:rw"},
		NetworkMode: "bridge",
		PortBindings: network.PortMap{
			port: []network.PortBinding{
				{HostIP: hostIP, HostPort: "5002"},
			},
		},
	}

	netCfg := &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			"bridge": {},
		},
	}

	if _, err := dc.ContainerCreate(ctx, cfg, hostCfg, netCfg, nil, "ella-core"); err != nil {
		return fmt.Errorf("container create ella-core: %w", err)
	}

	return nil
}

func (dc *DockerClient) ConnectContainerToNetwork(ctx context.Context, networkName string, containerName string, targetIP netip.Addr, ifname string) error {
	endpointCfg := &network.EndpointSettings{
		DriverOpts: map[string]string{
			"com.docker.network.endpoint.ifname": ifname,
		},
		IPAddress: targetIP,
	}

	if err := dc.NetworkConnect(ctx, networkName, containerName, endpointCfg); err != nil {
		return fmt.Errorf("failed to connect %s to %s: %w", containerName, networkName, err)
	}

	return nil
}

func (dc *DockerClient) StartContainer(ctx context.Context, containerName string) error {
	if err := dc.ContainerStart(ctx, containerName, client.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("failed to start container %q: %w", containerName, err)
	}

	return nil
}

func (dc *DockerClient) CreateUeransimContainer(ctx context.Context) error {
	cfg := &container.Config{
		Image: "ghcr.io/ellanetworks/ueransim:3.2.7",
	}
	host := &container.HostConfig{
		Privileged: true,
	}
	if _, err := dc.ContainerCreate(ctx, cfg, host, nil, nil, "ueransim"); err != nil {
		return fmt.Errorf("create ueransim: %w", err)
	}
	return nil
}

func (dc *DockerClient) CreateGnbsimContainer(ctx context.Context) error {
	cfg := &container.Config{
		Image: "ghcr.io/ellanetworks/sdcore-gnbsim:1.6.3",
	}
	host := &container.HostConfig{
		Privileged: true,
	}
	if _, err := dc.ContainerCreate(ctx, cfg, host, nil, nil, "gnbsim"); err != nil {
		return fmt.Errorf("create gnbsim: %w", err)
	}
	return nil
}

func (dc *DockerClient) CreateRouterContainer(ctx context.Context) error {
	cfg := &container.Config{
		Image: "ghcr.io/ellanetworks/ubuntu-router:0.1",
	}
	host := &container.HostConfig{
		Privileged: true,
	}
	if _, err := dc.ContainerCreate(ctx, cfg, host, nil, nil, "router"); err != nil {
		return fmt.Errorf("create router: %w", err)
	}
	return nil
}

func (dc *DockerClient) Exec(ctx context.Context, containerName string, command string, detach bool, timeout time.Duration, mirror io.Writer) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	execConfig := client.ExecCreateOptions{
		Cmd:          strings.Fields(command),
		AttachStdout: !detach,
		AttachStderr: !detach,
		Tty:          false,
		Privileged:   false,
	}

	execResp, err := dc.ContainerExecCreate(ctx, containerName, execConfig)
	if err != nil {
		return "", fmt.Errorf("exec create: %w", err)
	}

	if detach {
		if err := dc.ContainerExecStart(ctx, execResp.ID, client.ExecStartOptions{Detach: true}); err != nil {
			return "", fmt.Errorf("exec start (detached): %w", err)
		}
		return "", nil
	}

	attachResp, err := dc.ContainerExecAttach(ctx, execResp.ID, client.ExecStartOptions{})
	if err != nil {
		return "", fmt.Errorf("exec attach: %w", err)
	}
	defer attachResp.Close()

	var buf bytes.Buffer
	var writer io.Writer = &buf
	if mirror != nil {
		writer = io.MultiWriter(&buf, mirror)
	}

	if _, err := io.Copy(writer, attachResp.Reader); err != nil && ctx.Err() == nil {
		return buf.String(), fmt.Errorf("read exec output: %w", err)
	}

	inspect, err := dc.ContainerExecInspect(ctx, execResp.ID)
	if err != nil {
		return buf.String(), fmt.Errorf("inspect exec: %w", err)
	}

	if inspect.ExitCode != 0 {
		return buf.String(), fmt.Errorf("exec failed (exit %d):\n%s", inspect.ExitCode, buf.String())
	}

	return buf.String(), nil
}

func (dc *DockerClient) CleanUpDockerSpace(ctx context.Context) {
	// best-effort: ignore errors and keep going
	names := []string{"ella-core", "ueransim", "gnbsim", "router"}

	// Stop
	timeoutSec := 5
	for _, n := range names {
		_ = dc.ContainerStop(ctx, n, client.ContainerStopOptions{Timeout: &timeoutSec})
	}

	// Remove
	for _, n := range names {
		_ = dc.ContainerRemove(ctx, n, client.ContainerRemoveOptions{
			Force:         true,
			RemoveVolumes: true,
		})
	}

	// Networks
	for _, net := range []string{"n3", "n6"} {
		_ = dc.NetworkRemove(ctx, net)
	}
}

func (dc *DockerClient) CopyFileToContainer(ctx context.Context, containerName, srcPath, destPath string) error {
	f, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open %s: %w", srcPath, err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat %s: %w", srcPath, err)
	}

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	hdr := &tar.Header{
		Name:    path.Base(destPath),
		Mode:    0o644,
		Size:    info.Size(),
		ModTime: time.Now(),
	}

	if err := tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("tar header: %w", err)
	}

	if _, err := io.Copy(tw, f); err != nil {
		return fmt.Errorf("tar write: %w", err)
	}

	if err := tw.Close(); err != nil {
		return fmt.Errorf("tar close: %w", err)
	}

	dstDir := path.Dir(destPath)

	return dc.CopyToContainer(ctx, containerName, dstDir, &buf, client.CopyToContainerOptions{
		AllowOverwriteDirWithFile: true,
	})
}
