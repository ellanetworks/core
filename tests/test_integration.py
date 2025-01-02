#!/usr/bin/env python3

import logging
import subprocess
import time

import yaml

from tests.core import EllaCore, Subscriber

logger = logging.getLogger(__name__)

NAMESPACE = "dev2"
TEST_PROFILE_NAME = "default-default"
TEST_IMSI = "001010100007487"
NUM_PROFILES = 5


class TestELLA:
    async def test_given_ella_and_gnbsim_deployed_when_start_simulation_then_simulation_success_status_is_true(  # noqa: E501
        self,
    ):
        deploy_core_and_router_components()
        wait_for_ella_core_ready()
        configure_pebble_for_ella_core()
        time.sleep(10)
        core_port = get_core_node_port()
        core_address = f"https://127.0.0.1:{core_port}"
        subscriber = configure_ella_core(core_address=core_address)
        create_gnbsim_configmap_and_deployment(subscriber)
        time.sleep(10)
        success_runs = run_gnbsim_simulation(
            namespace=NAMESPACE,
            application_name="gnbsim",
            config_path="/etc/gnbsim/configuration.yaml",
            timeout=6 * 60,
        )
        assert success_runs == NUM_PROFILES


def configure_pebble_for_ella_core():
    """Add and start the Pebble layer for Ella Core."""
    try:
        logger.info("Configuring Pebble layer for Ella Core...")
        # Get the name of the Ella Core pod
        pod_name = subprocess.check_output(
            [
                "kubectl",
                "get",
                "pods",
                "-n",
                NAMESPACE,
                "-l",
                "app=ella-core",
                "-o",
                "jsonpath={.items[0].metadata.name}",
            ],
            text=True,
        ).strip()

        # Add the Pebble layer
        subprocess.check_call(
            [
                "kubectl",
                "exec",
                "-i",
                pod_name,
                "-n",
                NAMESPACE,
                "--",
                "pebble",
                "add",
                "ella-core",
                "/config/pebble.yaml",
            ]
        )
        logger.info("Pebble layer added successfully.")

        # Start the Pebble service
        subprocess.check_call(
            [
                "kubectl",
                "exec",
                "-i",
                pod_name,
                "-n",
                NAMESPACE,
                "--",
                "pebble",
                "start",
                "ella-core",
            ]
        )
        logger.info("Ella Core started successfully using Pebble.")
    except subprocess.CalledProcessError as e:
        logger.error(f"Failed to configure Pebble layer for Ella Core: {e}")
        raise RuntimeError("Failed to configure Pebble for Ella Core") from e


def deploy_core_and_router_components():
    """Deploy core and router components."""
    logger.info("Deploying core and router components...")
    manifests = [
        "k8s/router-ran-nad.yaml",
        "k8s/router-core-nad.yaml",
        "k8s/router-access-nad.yaml",
        "k8s/router-deployment.yaml",
        "k8s/core-n3-nad.yaml",
        "k8s/core-n6-nad.yaml",
        "k8s/core-configmap.yaml",
        "k8s/core-deployment.yaml",
        "k8s/core-service.yaml",
    ]
    for manifest in manifests:
        logger.info(f"Applying manifest: {manifest}")
        subprocess.check_call(["kubectl", "apply", "-f", manifest])
    logger.info("Core and router components deployed successfully.")


def wait_for_ella_core_ready():
    """Wait for Ella Core and Router components to be ready."""
    logger.info("Waiting for Ella Core and Router components to be ready...")

    components = {
        "ella-core": "app=ella-core",
        "router": "app=router",
    }

    for component_name, label_selector in components.items():
        try:
            logger.info(f"Waiting for {component_name} pods to be ready...")
            subprocess.check_call(
                [
                    "kubectl",
                    "wait",
                    "--namespace",
                    NAMESPACE,
                    "--for=condition=ready",
                    "pod",
                    "-l",
                    label_selector,
                    "--timeout=120s",
                ]
            )
            logger.info(f"{component_name} is ready.")
        except subprocess.CalledProcessError as e:
            logger.error(f"Timed out waiting for {component_name} to be ready: {e}")
            raise RuntimeError(f"{component_name} is not ready") from e


def create_gnbsim_configmap_and_deployment(subscriber: Subscriber):
    """Create GNBSim ConfigMap and deployment."""
    logger.info("Creating GNBSim ConfigMap and deployment...")
    push_config_file(subscriber)

    manifests = [
        "k8s/gnbsim-gnb-nad.yaml",
        "k8s/gnbsim-deployment.yaml",
        "k8s/gnbsim-service.yaml",
    ]
    for manifest in manifests:
        logger.info(f"Applying manifest: {manifest}")
        subprocess.check_call(["kubectl", "apply", "-f", manifest])
    logger.info("GNBSim components deployed successfully.")


def get_core_node_port() -> int:
    """Fetch the NodePort for the Ella Core service in the Kubernetes cluster."""
    try:
        node_port = subprocess.check_output(
            [
                "kubectl",
                "get",
                "service",
                "ella-core",
                "-n",
                NAMESPACE,
                "-o",
                "jsonpath={.spec.ports[0].nodePort}",
            ],
            text=True,
        ).strip()
        logger.info(f"Retrieved Ella Core NodePort: {node_port}")
        return int(node_port)
    except subprocess.CalledProcessError as e:
        logger.error(f"Failed to fetch Ella NodePort: {e.output}")
        raise RuntimeError("Could not retrieve NodePort for Ella service") from e


def configure_ella_core(core_address: str) -> Subscriber:
    """Configure Ella Core."""
    ella_client = EllaCore(url=core_address)
    ella_client.create_user(username="admin", password="admin")
    token = ella_client.login(username="admin", password="admin")
    if not token:
        raise RuntimeError("Failed to login to Ella Core")
    ella_client.set_token(token)
    ella_client.create_radio(name=f"{NAMESPACE}-gnbsim", tac="001")
    ella_client.create_profile(name=TEST_PROFILE_NAME)
    ella_client.create_subscriber(imsi=TEST_IMSI, profile_name=TEST_PROFILE_NAME)
    ella_client.update_operator_id()
    subscriber = ella_client.get_subscriber(imsi=TEST_IMSI)
    return subscriber


def push_config_file(subscriber: Subscriber) -> None:
    """Generate and create the GNBSim ConfigMap with the subscriber information."""
    logger.info("Creating GNBSim ConfigMap...")
    config = {
        "configuration.yaml": yaml.dump(
            {
                "configuration": {
                    "execInParallel": False,
                    "gnbs": {
                        "gnb1": {
                            "defaultAmf": {
                                "hostName": "ella-core.dev2.svc.cluster.local",
                                "port": 38412,
                            },
                            "globalRanId": {
                                "gNbId": {
                                    "bitLength": 24,
                                    "gNBValue": "000102",
                                },
                                "plmnId": {
                                    "mcc": "001",
                                    "mnc": "01",
                                },
                            },
                            "n2Port": 9487,
                            "n3IpAddr": "192.168.251.5",
                            "n3Port": 2152,
                            "name": "gnb1",
                            "supportedTaList": [
                                {
                                    "broadcastPlmnList": [
                                        {
                                            "plmnId": {
                                                "mcc": "001",
                                                "mnc": "01",
                                            },
                                            "taiSliceSupportList": [
                                                {
                                                    "sd": "102030",
                                                    "sst": 1,
                                                }
                                            ],
                                        }
                                    ],
                                    "tac": "000001",
                                }
                            ],
                        },
                    },
                    "profiles": [
                        {
                            "profileType": "register",
                            "profileName": "profile1",
                            "enable": True,
                            "gnbName": "gnb1",
                            "startImsi": subscriber.imsi,
                            "ueCount": 1,
                            "defaultAs": "192.168.250.1",
                            "opc": subscriber.opc,
                            "key": subscriber.key,
                            "sequenceNumber": subscriber.sequence_number,
                            "dnn": "internet",
                            "sNssai": {
                                "sst": 1,
                                "sd": "102030",
                            },
                            "plmnId": {
                                "mcc": "001",
                                "mnc": "01",
                            },
                        },
                    ],
                },
                "info": {
                    "description": "gNodeB sim initial configuration",
                    "version": "1.0.0",
                },
                "logger": {
                    "logLevel": "trace",
                },
            },
            default_flow_style=False,
        )
    }

    configmap_manifest = {
        "apiVersion": "v1",
        "kind": "ConfigMap",
        "metadata": {
            "name": "gnbsim-config",
            "namespace": NAMESPACE,
        },
        "data": config,
    }

    # Save ConfigMap manifest to a temporary file
    configmap_path = "/tmp/gnbsim-configmap.yaml"
    with open(configmap_path, "w") as f:
        yaml.dump(configmap_manifest, f)

    try:
        # Apply the ConfigMap manifest
        subprocess.check_call(["kubectl", "apply", "-f", configmap_path])
        logger.info("ConfigMap created successfully.")
    except subprocess.CalledProcessError as e:
        logger.error(f"Failed to create ConfigMap: {e}")
        raise RuntimeError("Failed to create ConfigMap for GNBSim") from e


def run_gnbsim_simulation(
    namespace: str, application_name: str, config_path: str, timeout: int
) -> int:
    """Run the GNBSim simulation."""
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
        logger.info(f"Running GNBSim simulation in pod {pod_name}")

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
            stderr=subprocess.STDOUT,
        ).strip()
        logger.info(f"GNBSim simulation output: {result}")
        return result.count("Profile Status: PASS")
    except subprocess.CalledProcessError as e:
        logger.error(f"GNBSim simulation failed: {e}")
        raise
