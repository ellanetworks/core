package integration_test

import (
	"fmt"
	"os/exec"
	"strings"

	"gopkg.in/yaml.v2"
)

type K8s struct {
	Namespace string
}

func (k *K8s) CreateNamespace() error {
	cmd := exec.Command("kubectl", "create", "namespace", k.Namespace)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create namespace %s: %s: %v", k.Namespace, string(out), err)
	}
	return nil
}

func (k *K8s) DeleteNamespace() error {
	cmd := exec.Command("kubectl", "delete", "namespace", k.Namespace)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to delete namespace %s: %s: %v", k.Namespace, string(out), err)
	}
	return nil
}

func (k *K8s) ApplyKustomize(kustomizeDir string) error {
	cmd := exec.Command("kubectl", "apply", "-k", kustomizeDir, "-n", k.Namespace)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("kubectl apply failed: %s: %v", string(out), err)
	}
	return nil
}

func (k *K8s) WaitForAppReady(name string) error {
	labelSelector := "app=" + name
	cmd := exec.Command("kubectl", "wait", "--for=condition=ready", "--timeout=120s", "pod", "-l", labelSelector, "-n", k.Namespace)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("kubectl wait failed: %s: %v", string(out), err)
	}
	return nil
}

func (k *K8s) GetNodePort(serviceName string) (int, error) {
	cmd := exec.Command("kubectl", "get", "service", serviceName, "-n", k.Namespace, "-o", "jsonpath='{.spec.ports[0].nodePort}'")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("kubectl get service failed: %s: %v", string(out), err)
	}
	var nodePort int
	_, err = fmt.Sscanf(string(out), "'%d'", &nodePort)
	if err != nil {
		return 0, fmt.Errorf("failed to parse node port: %v", err)
	}
	return nodePort, nil
}

func (k *K8s) GetConfigMap(name string) (map[string]interface{}, error) {
	cmd := exec.Command("kubectl", "get", "configmap", name, "-n", k.Namespace, "-o", "yaml")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("kubectl get configmap failed: %s: %v", string(out), err)
	}
	var configMap map[string]interface{}
	err = yaml.Unmarshal(out, &configMap)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal configmap: %v", err)
	}
	return configMap, nil
}

func (k *K8s) PatchConfigMap(name string, patch map[string]interface{}) error {
	patchYaml, err := yaml.Marshal(patch)
	if err != nil {
		return fmt.Errorf("failed to marshal patch: %v", err)
	}
	cmd := exec.Command("kubectl", "patch", "configmap", name, "-n", k.Namespace, "--patch", string(patchYaml))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("kubectl patch failed: %s: %v", string(out), err)
	}
	return nil
}

func (k *K8s) RolloutRestart(deploymentName string) error {
	cmd := exec.Command("kubectl", "rollout", "restart", "deployment", deploymentName, "-n", k.Namespace)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("kubectl rollout restart failed: %s: %v", string(out), err)
	}
	return nil
}

func (k *K8s) WaitForRollout(deploymentName string) error {
	cmd := exec.Command("kubectl", "rollout", "status", "deployment", deploymentName, "-n", k.Namespace)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("kubectl rollout status failed: %s: %v", string(out), err)
	}
	return nil
}

func (k *K8s) GetPodName(appName string) (string, error) {
	cmd := exec.Command("kubectl", "get", "pods", "-n", k.Namespace, "-l", fmt.Sprintf("app=%s", appName), "--sort-by=.metadata.creationTimestamp", "-o", "jsonpath={.items[*].metadata.name}")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("kubectl get pods failed: %s: %v", string(out), err)
	}
	podNames := string(out)
	if podNames == "" {
		return "", fmt.Errorf("no pods found")
	}
	parts := strings.Fields(podNames)
	if len(parts) == 0 {
		return "", fmt.Errorf("no pods found")
	}
	podName := parts[len(parts)-1]
	return podName, nil
}

func (k *K8s) Exec(podName string, command string, container string) (string, error) {
	baseArgs := []string{
		"exec",
		"-i",
		podName,
		"-c", container,
		"-n", k.Namespace,
		"--",
	}
	cmdArgs := strings.Fields(command)
	args := append(baseArgs, cmdArgs...)

	cmd := exec.Command("kubectl", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("kubectl exec failed: %s: %v", string(out), err)
	}
	return string(out), nil
}
