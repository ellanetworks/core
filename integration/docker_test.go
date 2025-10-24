package integration_test

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
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

func dockerExec(containerName string, command string, detach bool) (string, error) {
	args := []string{"exec"}
	if detach {
		args = append(args, "-d")
	} else {
		args = append(args, "-i")
	}

	args = append(args, containerName)

	args = append(args, strings.Fields(command)...)

	cmd := exec.Command("docker", args...)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("docker exec failed: %s: %w", string(out), err)
	}

	return string(out), nil
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
