package integration_test

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/moby/moby/client"
)

type DockerClient struct {
	*client.Client
}

func NewDockerClient() (*DockerClient, error) {
	cli, err := client.New(client.FromEnv)
	if err != nil {
		return nil, fmt.Errorf("create docker client: %w", err)
	}

	return &DockerClient{Client: cli}, nil
}

// ComposeUp starts containers defined in a docker-compose file located in composeDir
// Note: `compose` is not part of the moby client, so we use exec.Command to call the CLI
func (dc *DockerClient) ComposeUp(ctx context.Context, composeDir string) error {
	cmd := exec.CommandContext(ctx, "docker", "compose", "up", "-d")
	cmd.Dir = composeDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run docker compose up: %w", err)
	}

	return nil
}

// ComposeDown stops and removes containers defined in a docker-compose file located in composeDir
// Note: `compose` is not part of the moby client, so we use exec.Command to call the CLI.
// Volumes are removed so test runs start from fresh state; HA tests rely on this
// because a stale ella.db or raft log from a previous run would make a node think
// it has already bootstrapped and skip the discovery/join path.
func (dc *DockerClient) ComposeDown(ctx context.Context, composeDir string) {
	cmd := exec.CommandContext(ctx, "docker", "compose", "down", "-v")
	cmd.Dir = composeDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	_ = cmd.Run()
}

func (dc *DockerClient) ResolveComposeContainer(ctx context.Context, project, service string) (string, error) {
	f := client.Filters{}
	f.Add("label", "com.docker.compose.project="+project)
	f.Add("label", "com.docker.compose.service="+service)

	cs, err := dc.ContainerList(ctx, client.ContainerListOptions{All: true, Filters: f})
	if err != nil {
		return "", fmt.Errorf("list containers: %w", err)
	}

	if len(cs.Items) == 0 {
		return "", fmt.Errorf("no container found for project=%q service=%q", project, service)
	}

	// Prefer the human-readable name
	if len(cs.Items[0].Names) > 0 {
		name := strings.TrimPrefix(cs.Items[0].Names[0], "/")
		return name, nil
	}

	return cs.Items[0].ID, nil
}

func (dc *DockerClient) Exec(ctx context.Context, containerName string, argv []string, detach bool, timeout time.Duration, mirror io.Writer) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	execConfig := client.ExecCreateOptions{
		Cmd:          argv,
		AttachStdout: !detach,
		AttachStderr: !detach,
		TTY:          false,
		Privileged:   false,
	}

	execResp, err := dc.ExecCreate(ctx, containerName, execConfig)
	if err != nil {
		return "", fmt.Errorf("exec create: %w", err)
	}

	if detach {
		if _, err := dc.ExecStart(ctx, execResp.ID, client.ExecStartOptions{Detach: true}); err != nil {
			return "", fmt.Errorf("exec start (detached): %w", err)
		}

		return "", nil
	}

	attachResp, err := dc.ExecAttach(ctx, execResp.ID, client.ExecAttachOptions{})
	if err != nil {
		return "", fmt.Errorf("exec attach: %w", err)
	}
	defer attachResp.Close()

	var (
		buf    bytes.Buffer
		writer io.Writer = &buf
	)

	if mirror != nil {
		writer = io.MultiWriter(&buf, mirror)
	}

	if _, err := io.Copy(writer, attachResp.Reader); err != nil && ctx.Err() == nil {
		return buf.String(), fmt.Errorf("read exec output: %w", err)
	}

	inspect, err := dc.ExecInspect(ctx, execResp.ID, client.ExecInspectOptions{})
	if err != nil {
		return buf.String(), fmt.Errorf("inspect exec: %w", err)
	}

	if inspect.ExitCode != 0 {
		return buf.String(), fmt.Errorf("exec failed (exit %d):\n%s", inspect.ExitCode, buf.String())
	}

	return buf.String(), nil
}

// ComposeLogs returns the logs of a specific service in a docker-compose project.
func (dc *DockerClient) ComposeLogs(ctx context.Context, composeDir string, service string) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", "compose", "logs", "--no-color", service)
	cmd.Dir = composeDir

	var buf bytes.Buffer

	cmd.Stdout = &buf
	cmd.Stderr = &buf

	if err := cmd.Run(); err != nil {
		return buf.String(), fmt.Errorf("failed to run docker compose logs: %w", err)
	}

	return buf.String(), nil
}

func (dc *DockerClient) CopyFileToContainer(ctx context.Context, containerName, srcPath, destPath string) error {
	f, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open %s: %w", srcPath, err)
	}

	defer func() {
		err := f.Close()
		if err != nil {
			fmt.Printf("warning: could not close file %s: %v\n", srcPath, err)
		}
	}()

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

	_, err = dc.CopyToContainer(ctx, containerName, client.CopyToContainerOptions{
		DestinationPath:           dstDir,
		Content:                   &buf,
		AllowOverwriteDirWithFile: true,
	})
	if err != nil {
		return fmt.Errorf("copy to container: %w", err)
	}

	return nil
}

func (dc *DockerClient) ComposeStop(ctx context.Context, composeDir string, service string) error {
	cmd := exec.CommandContext(ctx, "docker", "compose", "stop", service)
	cmd.Dir = composeDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stop service %s: %w", service, err)
	}

	return nil
}

func (dc *DockerClient) ComposeStart(ctx context.Context, composeDir string, service string) error {
	cmd := exec.CommandContext(ctx, "docker", "compose", "start", service)
	cmd.Dir = composeDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start service %s: %w", service, err)
	}

	return nil
}

// ComposeUpServices creates and starts only the named services from a compose
// file. Use this when a compose file defines more services than should run
// initially (e.g. scale-up tests that add nodes later).
func (dc *DockerClient) ComposeUpServices(ctx context.Context, composeDir string, services ...string) error {
	args := append([]string{"compose", "up", "-d"}, services...)
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = composeDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run docker compose up %v: %w", services, err)
	}

	return nil
}

// ComposeCreate creates containers for the named services without starting
// them. Used by HA tests that need to seed join-token files into a
// follower's data dir before the daemon comes up.
func (dc *DockerClient) ComposeCreate(ctx context.Context, composeDir string, services ...string) error {
	args := append([]string{"compose", "create"}, services...)
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = composeDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run docker compose create %v: %w", services, err)
	}

	return nil
}

// CopyBytesToContainer writes data to destPath inside containerName.
// destPath must include the filename; its parent directory must already
// exist in the container.
func (dc *DockerClient) CopyBytesToContainer(ctx context.Context, containerName string, data []byte, destPath string, mode int64) error {
	var buf bytes.Buffer

	tw := tar.NewWriter(&buf)

	hdr := &tar.Header{
		Name:    path.Base(destPath),
		Mode:    mode,
		Size:    int64(len(data)),
		ModTime: time.Now(),
	}

	if err := tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("tar header: %w", err)
	}

	if _, err := tw.Write(data); err != nil {
		return fmt.Errorf("tar write: %w", err)
	}

	if err := tw.Close(); err != nil {
		return fmt.Errorf("tar close: %w", err)
	}

	_, err := dc.CopyToContainer(ctx, containerName, client.CopyToContainerOptions{
		DestinationPath:           path.Dir(destPath),
		Content:                   &buf,
		AllowOverwriteDirWithFile: true,
	})
	if err != nil {
		return fmt.Errorf("copy to container: %w", err)
	}

	return nil
}
