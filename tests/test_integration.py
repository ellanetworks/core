#!/usr/bin/env python3
# Copyright 2024 Guillaume Belanger
# See LICENSE file for licensing details.

import json
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
        core_port = get_core_node_port()
        core_address = f"https://127.0.0.1:{core_port}"
        subscriber = configure_ella_core(core_address=core_address)
        push_config_file(subscriber)
        time.sleep(20)
        success_runs = run_gnbsim_simulation(
            namespace=NAMESPACE,
            application_name="gnbsim",
            config_path="/etc/gnbsim/configuration.yaml",
            timeout=6 * 60,
        )
        assert success_runs == NUM_PROFILES


def get_core_node_port() -> int:
    """Fetch the NodePort for the Ella Core service in the Kubernetes cluster.

    Returns:
        int: The NodePort of the Ella Core service.

    Raises:
        RuntimeError: If the NodePort cannot be retrieved.
    """
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
    except ValueError as e:
        logger.error(f"NodePort value is invalid: {e}")
        raise RuntimeError("Invalid NodePort value retrieved") from e


def configure_ella_core(core_address: str) -> Subscriber:
    """Configure Ella Core.

    Configuration includes:
    - subscriber creation
    - profile creation
    - operator configuration update
    """
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


def run_gnbsim_simulation(
    namespace: str, application_name: str, config_path: str, timeout: int
) -> int:
    """Run the GNBSim simulation command in the container.

    Args:
        namespace (str): Kubernetes namespace.
        application_name (str): Application name (K8s deployment name).
        container_name (str): Container name in the pod.
        config_path (str): Path to the GNBSim configuration file.
        timeout (int): Maximum timeout for the command execution in seconds.

    Returns:
        int: Number of successful profile runs.
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
        return 0
    logger.info(f"Running GNBSim simulation in pod {pod_name}")

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
            stderr=subprocess.STDOUT,
        ).strip()
        logger.info(f"GNBSim simulation output: {result}")
        # Count the number of times `Profile Status: PASS` appears
        return result.count("Profile Status: PASS")
    except subprocess.CalledProcessError:
        return 0


def push_config_file(subscriber: Subscriber) -> None:
    """Generate the configuration file for GNBSim and push it to the container.

    Args:
        subscriber (Subscriber): The subscriber information.
    """
    config = {
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
            "goProfile": {
                "enable": False,
                "port": 5000,
            },
            "httpServer": {
                "enable": False,
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
                {
                    "profileType": "pdusessest",
                    "profileName": "profile2",
                    "enable": True,
                    "gnbName": "gnb1",
                    "dataPktCount": 5,
                    "defaultAs": "192.168.250.1",
                    "opc": subscriber.opc,
                    "key": subscriber.key,
                    "perUserTimeout": 100,
                    "dnn": "internet",
                    "sNssai": {
                        "sst": 1,
                        "sd": "102030",
                    },
                    "plmnId": {
                        "mcc": "001",
                        "mnc": "01",
                    },
                    "sequenceNumber": subscriber.sequence_number,
                    "startImsi": subscriber.imsi,
                    "ueCount": 1,
                },
                {
                    "profileType": "anrelease",
                    "profileName": "profile3",
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
                    "execInParallel": False,
                    "plmnId": {
                        "mcc": "001",
                        "mnc": "01",
                    },
                },
                {
                    "profileType": "uetriggservicereq",
                    "profileName": "profile4",
                    "enable": True,
                    "gnbName": "gnb1",
                    "startImsi": subscriber.imsi,
                    "ueCount": 1,
                    "defaultAs": "192.168.250.1",
                    "opc": subscriber.opc,
                    "key": subscriber.key,
                    "sequenceNumber": subscriber.sequence_number,
                    "dnn": "internet",
                    "retransMsg": False,
                    "sNssai": {
                        "sst": 1,
                        "sd": "102030",
                    },
                    "execInParallel": False,
                    "plmnId": {
                        "mcc": "001",
                        "mnc": "01",
                    },
                },
                {
                    "profileType": "deregister",
                    "profileName": "profile5",
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
                    "execInParallel": False,
                    "plmnId": {
                        "mcc": "001",
                        "mnc": "01",
                    },
                },
            ],
            "runConfigProfilesAtStart": True,
        },
        "info": {
            "description": "gNodeB sim initial configuration",
            "version": "1.0.0",
        },
        "logger": {
            "logLevel": "trace",
        },
    }

    config_yaml = yaml.dump(config, default_flow_style=False)

    patch_payload = json.dumps(
        [
            {
                "op": "replace",
                "path": "/data/configuration.yaml",
                "value": config_yaml,
            }
        ]
    )

    try:
        subprocess.check_call(
            [
                "kubectl",
                "patch",
                "configmap",
                "gnbsim-config",
                "-n",
                NAMESPACE,
                "--type=json",
                "-p",
                patch_payload,
            ],
            stderr=subprocess.STDOUT,
        )
        logger.info("ConfigMap updated successfully.")
    except subprocess.CalledProcessError as e:
        logger.error(f"Failed to update ConfigMap: {e}")
        raise RuntimeError("Failed to update ConfigMap for GNBSim") from e

def restart_pod(app_name: str):
    subprocess.check_call(
        [
            "kubectl",
            "rollout",
            "restart",
            f"deployment/{app_name}",
            "-n",
            NAMESPACE,
        ]
    )
    logger.info(f"Restarted deployment {app_name} in namespace {NAMESPACE}")


def print_configmap():
    config_data = subprocess.check_output(
        [
            "kubectl",
            "get",
            "configmap",
            "gnbsim-config",
            "-n",
            NAMESPACE,
            "-o",
            "jsonpath={.data.configuration\\.yaml}",
        ],
        text=True,
    )
    config = yaml.safe_load(config_data)
    print(json.dumps(config, indent=2))
