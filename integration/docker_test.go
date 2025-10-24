package integration_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

func createN3Network() error {
	cmd := exec.Command("docker", "network", "create", "--driver", "bridge", "n3", "--subnet", "10.3.0.0/24")

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create network n3: %s: %v", string(out), err)
	}

	return nil
}

func createN6Network() error {
	cmd := exec.Command("docker", "network", "create", "--driver", "bridge", "n6", "--subnet", "10.6.0.0/24")

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create network n6: %s: %v", string(out), err)
	}

	return nil
}

func createEllaCoreContainerWithConfig(configPath string) error {
	configPath, err := filepath.Abs(configPath)
	if err != nil {
		return fmt.Errorf("resolve config path: %w", err)
	}

	cmd := exec.Command("docker", "create",
		"--name", "ella-core",
		"--privileged",
		"--network", "name=bridge",
		"-p", "5002:5002",
		"-v", configPath+":/core.yaml:ro",
		"-v", "/sys/fs/bpf:/sys/fs/bpf:rw",
		"ella-core:latest",
		"exec", "/bin/core", "--config", "/core.yaml",
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create ella-core container: %s: %v", string(out), err)
	}

	return nil
}

func connectEllaCoreToN3() error {
	cmd := exec.Command("docker", "network", "connect",
		"--driver-opt", "com.docker.network.endpoint.ifname=n3",
		"--ip", "10.3.0.2",
		"n3", "ella-core",
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to connect ella-core to n3: %s: %v", string(out), err)
	}

	return nil
}

func connectEllaCoreToN6() error {
	cmd := exec.Command("docker", "network", "connect",
		"--driver-opt", "com.docker.network.endpoint.ifname=n6",
		"--ip", "10.6.0.2",
		"n6", "ella-core",
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to connect ella-core to n6: %s: %v", string(out), err)
	}

	return nil
}

func startEllaCoreContainer() error {
	cmd := exec.Command("docker", "start", "ella-core")

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to start ella-core container: %s: %v", string(out), err)
	}

	return nil
}

func createUeransimContainer() error {
	cmd := exec.Command("docker", "create",
		"--name", "ueransim",
		"--privileged",
		"ghcr.io/ellanetworks/ueransim:3.2.7",
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create ueransim container: %s: %v", string(out), err)
	}

	return nil
}

func connectUeransimToN3() error {
	cmd := exec.Command("docker", "network", "connect",
		"--driver-opt", "com.docker.network.endpoint.ifname=n3",
		"--ip", "10.3.0.3",
		"n3", "ueransim",
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to connect ueransim to n3: %s: %v", string(out), err)
	}

	return nil
}

func startUeransimContainer() error {
	cmd := exec.Command("docker", "start", "ueransim")

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to start ueransim container: %s: %v", string(out), err)
	}

	return nil
}

func createGnbsimContainer() error {
	cmd := exec.Command("docker", "create",
		"--name", "gnbsim",
		"--privileged",
		"ghcr.io/ellanetworks/sdcore-gnbsim:1.6.3",
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create gnbsim container: %s: %v", string(out), err)
	}

	return nil
}

func connectGnbsimToN3() error {
	cmd := exec.Command("docker", "network", "connect",
		"--driver-opt", "com.docker.network.endpoint.ifname=n3",
		"--ip", "10.3.0.3",
		"n3", "gnbsim",
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to connect gnbsim to n3: %s: %v", string(out), err)
	}

	return nil
}

func startGnbsimContainer() error {
	cmd := exec.Command("docker", "start", "gnbsim")

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to start gnbsim container: %s: %v", string(out), err)
	}

	return nil
}

func createRouterContainer() error {
	cmd := exec.Command(
		"docker", "create",
		"--name", "router",
		"--privileged",
		"ghcr.io/ellanetworks/ubuntu-router:0.1",
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create router container: %s: %v", string(out), err)
	}

	return nil
}

func connectRouterToN6() error {
	cmd := exec.Command("docker", "network", "connect",
		"--driver-opt", "com.docker.network.endpoint.ifname=n6",
		"--ip", "10.6.0.3",
		"n6", "router",
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to connect gnbsim to n3: %s: %v", string(out), err)
	}

	return nil
}

func startRouterContainer() error {
	cmd := exec.Command("docker", "start", "router")

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to start router container: %s: %v", string(out), err)
	}

	return nil
}

func dockerExec(ctx context.Context, containerName, command string, detach bool, timeout time.Duration, mirror io.Writer) (string, error) {
	args := []string{"exec"}
	if detach {
		args = append(args, "-d")
	}
	args = append(args, containerName)
	args = append(args, strings.Fields(command)...)

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("stderr pipe: %w", err)
	}

	var buf bytes.Buffer
	var wg sync.WaitGroup
	wg.Add(2)

	// tee to buffer and optional mirror
	copyTo := func(r io.Reader) {
		defer wg.Done()
		if mirror != nil {
			_, _ = io.Copy(io.MultiWriter(&buf, mirror), r)
		} else {
			_, _ = io.Copy(&buf, r)
		}
	}

	go copyTo(stdout)
	go copyTo(stderr)

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("start: %w", err)
	}

	waitErr := cmd.Wait()
	wg.Wait()

	out := buf.String()

	if ctx.Err() == context.DeadlineExceeded {
		return out, fmt.Errorf("docker exec timed out after %v; output:\n%s", timeout, out)
	}
	if waitErr != nil {
		return out, fmt.Errorf("docker exec failed: %w\noutput:\n%s", waitErr, out)
	}
	return out, nil
}

func cleanUpDockerSpace() {
	cmd := exec.Command("docker", "stop", "ella-core", "ueransim", "gnbsim", "router")
	_, _ = cmd.CombinedOutput()

	cmd = exec.Command("docker", "rm", "ella-core", "ueransim", "gnbsim", "router")
	_, _ = cmd.CombinedOutput()

	cmd = exec.Command("docker", "network", "rm", "n3", "n6")
	_, _ = cmd.CombinedOutput()
}

func copyTestingScript() error {
	cmd := exec.Command("docker", "cp", "network_test.py", "ueransim:/network_test.py")

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to copy testing script to ueransim container: %s: %v", string(out), err)
	}

	return nil
}
