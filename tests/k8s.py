#!/usr/bin/env python3
import logging
import subprocess

import yaml

logger = logging.getLogger(__name__)


class Kubernetes:
    def __init__(self, namespace: str):
        self.namespace = namespace

    def create_namespace(self):
        try:
            subprocess.check_call(["kubectl", "create", "namespace", self.namespace])
        except subprocess.CalledProcessError as e:
            logger.error(f"Failed to create namespace: {e}")
            raise RuntimeError("Failed to create namespace") from e

    def delete_namespace(self):
        try:
            subprocess.check_call(["kubectl", "delete", "namespace", self.namespace])
        except subprocess.CalledProcessError as e:
            logger.error(f"Failed to delete namespace: {e}")
            raise RuntimeError("Failed to delete namespace") from e

    def get_configmap(self, name: str) -> dict:
        """Fetch the given ConfigMap using kubectl."""
        cmd = [
            "kubectl",
            "get",
            "configmap",
            name,
            "-n",
            self.namespace,
            "-o",
            "yaml",
        ]
        output = subprocess.check_output(cmd)
        return yaml.safe_load(output)

    def patch_configmap(self, name: str, patch: dict) -> None:
        """Patch the given ConfigMap using kubectl."""
        patch_yaml = yaml.dump(patch, default_flow_style=False)
        cmd = [
            "kubectl",
            "patch",
            "configmap",
            name,
            "-n",
            self.namespace,
            "--patch",
            patch_yaml,
        ]
        subprocess.check_call(cmd)

    def apply_manifest(self, manifest_path: str):
        subprocess.check_call(["kubectl", "apply", "-f", manifest_path, "-n", self.namespace])

    def apply_kustomize(self, kustomize_path: str):
        subprocess.check_call(["kubectl", "apply", "-k", kustomize_path, "-n", self.namespace])

    def wait_for_app_ready(self, app_name: str):
        label_selector = f"app={app_name}"
        try:
            logger.info(f"Waiting for {app_name} pods to be ready...")
            subprocess.check_call(
                [
                    "kubectl",
                    "wait",
                    "--namespace",
                    self.namespace,
                    "--for=condition=ready",
                    "pod",
                    "-l",
                    label_selector,
                    "--timeout=120s",
                ]
            )
            logger.info(f"{app_name} is ready.")
        except subprocess.CalledProcessError as e:
            logger.error(f"Timed out waiting for {app_name} to be ready: {e}")
            raise RuntimeError(f"{app_name} is not ready") from e

    def get_pod_name(self, app_name: str):
        try:
            # Get all pod names sorted by creation timestamp
            pod_names_str = subprocess.check_output(
                [
                    "kubectl",
                    "get",
                    "pods",
                    "-n",
                    self.namespace,
                    "-l",
                    f"app={app_name}",
                    "--sort-by=.metadata.creationTimestamp",
                    "-o",
                    "jsonpath={.items[*].metadata.name}",
                ],
                text=True,
            ).strip()
            pod_names = pod_names_str.split()
            if not pod_names:
                raise RuntimeError("No pods found")
            # Return the last pod (the newest one)
            return pod_names[-1]
        except subprocess.CalledProcessError as e:
            logger.error(f"Failed to get pod name: {e}")
            raise RuntimeError("Failed to get pod name") from e

    def get_core_node_port(self, service_name: str) -> int:
        """Fetch the NodePort for the Ella Core service in the Kubernetes cluster."""
        try:
            node_port = subprocess.check_output(
                [
                    "kubectl",
                    "get",
                    "service",
                    service_name,
                    "-n",
                    self.namespace,
                    "-o",
                    "jsonpath={.spec.ports[0].nodePort}",
                ],
                text=True,
            ).strip()
            logger.info(f"Retrieved {service_name} NodePort: {node_port}")
            return int(node_port)
        except subprocess.CalledProcessError as e:
            logger.error(f"Failed to fetch Ella NodePort: {e.output}")
            raise RuntimeError(f"Could not retrieve NodePort for {service_name} service") from e

    def rollout_restart(self, deployment_name: str):
        """Rollout restart the given deployment using kubectl."""
        try:
            subprocess.check_call(
                [
                    "kubectl",
                    "rollout",
                    "restart",
                    "deployment",
                    deployment_name,
                    "-n",
                    self.namespace,
                ]
            )
        except subprocess.CalledProcessError as e:
            logger.error(f"Failed to rollout restart: {e}")
            raise RuntimeError("Failed to rollout restart") from e

    def wait_for_rollout(self, deployment_name: str):
        """Wait for the rollout of the given deployment to finish."""
        try:
            subprocess.check_call(
                [
                    "kubectl",
                    "rollout",
                    "status",
                    "deployment",
                    deployment_name,
                    "-n",
                    self.namespace,
                ]
            )
        except subprocess.CalledProcessError as e:
            logger.error(f"Failed to wait for rollout: {e}")
            raise RuntimeError("Failed to wait for rollout") from e

    def exec(
        self,
        pod_name: str,
        command: str,
        container: str,
        timeout: int = 60,
    ) -> str:
        command_list = command.split()
        try:
            result = subprocess.check_output(
                [
                    "kubectl",
                    "exec",
                    "-i",
                    pod_name,
                    "-c",
                    container,
                    "-n",
                    self.namespace,
                    "--",
                    *command_list,
                ],
                timeout=timeout,
                text=True,
                stderr=subprocess.STDOUT,
            )
            return result.strip()
        except subprocess.CalledProcessError as e:
            logger.error(f"Command failed with exit code {e.returncode}: {e.output}")
            return ""
