#!/usr/bin/env python3
# Copyright 2024 Guillaume Belanger
# See LICENSE file for licensing details.

import logging
import subprocess
import time

from tests.ella import Ella

logger = logging.getLogger(__name__)

NAMESPACE = "dev2"
TEST_DEVICE_GROUP_NAME = "default-default"
TEST_IMSI = "001010100007487"
TEST_NETWORK_SLICE_NAME = "default"


class TestELLA:
    async def test_given_sdcore_bundle_and_gnbsim_deployed_when_start_simulation_then_simulation_success_status_is_true(  # noqa: E501
        self,
    ):
        ella_port = get_ella_node_port()
        ella_address = f"http://127.0.0.1:{ella_port}"
        configure_sdcore(ella_address=ella_address)
        for _ in range(5):
            action_output = run_gnbsim_simulation(
                namespace=NAMESPACE,
                application_name="gnbsim",
                config_path="/etc/gnbsim/configuration.yaml",
                timeout=6 * 60,
            )
            try:
                assert action_output["success"] == "true"
                return
            except AssertionError:
                time.sleep(1)
                continue
        assert False


def get_ella_node_port() -> int:
    """Fetch the NodePort for the Ella service in the Kubernetes cluster.

    Returns:
        int: The NodePort of the Ella service.

    Raises:
        RuntimeError: If the NodePort cannot be retrieved.
    """
    try:
        node_port = subprocess.check_output(
            [
                "kubectl",
                "get",
                "service",
                "ella",
                "-n",
                NAMESPACE,
                "-o",
                "jsonpath={.spec.ports[0].nodePort}",
            ],
            text=True,
        ).strip()
        logger.info(f"Retrieved Ella NodePort: {node_port}")
        return int(node_port)
    except subprocess.CalledProcessError as e:
        logger.error(f"Failed to fetch Ella NodePort: {e.output}")
        raise RuntimeError("Could not retrieve NodePort for Ella service") from e
    except ValueError as e:
        logger.error(f"NodePort value is invalid: {e}")
        raise RuntimeError("Invalid NodePort value retrieved") from e


def configure_sdcore(ella_address: str) -> None:
    """Configure Charmed SD-Core.

    Configuration includes:
    - subscriber creation
    - device group creation
    - network slice creation
    """
    ella_client = Ella(url=ella_address)
    ella_client.create_subscriber(imsi=TEST_IMSI)
    ella_client.create_device_group(name=TEST_DEVICE_GROUP_NAME, imsis=[TEST_IMSI])
    ella_client.create_network_slice(
        name=TEST_NETWORK_SLICE_NAME,
        device_groups=[TEST_DEVICE_GROUP_NAME],
    )
    time.sleep(5)


def run_gnbsim_simulation(namespace: str, application_name: str, config_path: str, timeout: int):
    """Run the GNBSim simulation command in the container.

    Args:
        namespace (str): Kubernetes namespace.
        application_name (str): Application name (K8s deployment name).
        container_name (str): Container name in the pod.
        config_path (str): Path to the GNBSim configuration file.
        timeout (int): Maximum timeout for the command execution in seconds.

    Returns:
        dict: Simulation result.
    """
    try:
        pod_name = subprocess.check_output(
            [
                "kubectl",
                "get",
                "pods",
                "-n",
                namespace,
                "-l",
                f"app={application_name}",
                "-o",
                "jsonpath={.items[0].metadata.name}",
            ],
            text=True,
        ).strip()
    except subprocess.CalledProcessError as e:
        logger.error(f"Failed to get pod name for {application_name}: {e}")
        raise

    try:
        result = subprocess.check_output(
            [
                "kubectl",
                "exec",
                "-n",
                namespace,
                pod_name,
                "--",
                "pebble",
                "exec",
                "gnbsim",
                "--cfg",
                config_path,
            ],
            text=True,
            timeout=timeout,
        ).strip()
        logger.info(f"GNBSim simulation output: {result}")
        return {"success": "true", "output": result}
    except subprocess.CalledProcessError as e:
        logger.error(f"GNBSim simulation failed: {e.output}")
        return {"success": "false", "error": str(e)}
