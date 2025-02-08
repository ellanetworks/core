#!/usr/bin/env python3

import logging
import subprocess

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

    def apply_manifest(self, manifest_path: str):
        subprocess.check_call(["kubectl", "apply", "-f", manifest_path, "-n", self.namespace])

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
            pod_name = subprocess.check_output(
                [
                    "kubectl",
                    "get",
                    "pods",
                    "-n",
                    self.namespace,
                    "-l",
                    f"app={app_name}",
                    "-o",
                    "jsonpath={.items[0].metadata.name}",
                ],
                text=True,
            ).strip()
            return pod_name
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

    def exec(self, pod_name: str, command: str, timeout: int = 60) -> str:
        command_list = command.split()
        try:
            result = subprocess.check_output(
                [
                    "kubectl",
                    "exec",
                    "-i",
                    pod_name,
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
