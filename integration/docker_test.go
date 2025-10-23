package integration_test

import (
	"fmt"
	"os/exec"
)

func createN3Network() error {
	cmd := exec.Command("docker", "network", "create", "--driver", "bridge", "n3", "--subnet", "10.3.0.0/24")

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create network n3: %s: %v", string(out), err)
	}

	return nil
}

func createEllaCoreContainer() error {
	cmd := exec.Command("docker", "create",
		"--name", "ella-core",
		"--privileged",
		"--network", "name=bridge",
		"-p", "5002:5002",
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
		"gnbsim:latest",
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

func dockerExec(containerName string, command string, detach bool) (string, error) {
	args := []string{"exec"}
	if detach {
		args = append(args, "-d")
	} else {
		args = append(args, "-i")
	}
	args = append(args, containerName)

	args = append(args, "sh", "-lc", command)

	cmd := exec.Command("docker", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("docker exec failed: %s: %w", string(out), err)
	}
	return string(out), nil
}

func cleanUpDockerSpace() {
	cmd := exec.Command("docker", "stop", "ella-core", "ueransim", "gnbsim")
	_, _ = cmd.CombinedOutput()

	cmd = exec.Command("docker", "rm", "ella-core", "ueransim", "gnbsim")
	_, _ = cmd.CombinedOutput()

	cmd = exec.Command("docker", "network", "rm", "n3")
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
