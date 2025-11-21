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
	cli, err := client.New(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("create docker client: %w", err)
	}

	return &DockerClient{Client: cli}, nil
}

// ComposeUp starts containers defined in a docker-compose file located in composeDir
// Note: `compose` is not part of the moby client, so we use exec.Command to call the CLI
func (dc *DockerClient) ComposeUp(composeDir string) error {
	cmd := exec.Command("docker", "compose", "up", "-d")
	cmd.Dir = composeDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run docker compose up: %w", err)
	}

	return nil
}

// ComposeDown stops and removes containers defined in a docker-compose file located in composeDir
// Note: `compose` is not part of the moby client, so we use exec.Command to call the CLI
func (dc *DockerClient) ComposeDown(composeDir string) {
	cmd := exec.Command("docker", "compose", "down")
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

	var buf bytes.Buffer
	var writer io.Writer = &buf
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
