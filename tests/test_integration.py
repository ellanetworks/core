#!/usr/bin/env python3

import logging
import time
from typing import Tuple

import yaml

from tests.core import EllaCore, Subscriber
from tests.k8s import Kubernetes

logger = logging.getLogger(__name__)

GNBSIM_NAMESPACE = "gnbsim"
UERANSIM_NAMESPACE = "ueransim"
TEST_PROFILE_NAME = "default"
TEST_START_IMSI = "001010100000001"
NUM_IMSIS = 5
TEST_SUBSCRIBER_KEY = "5122250214c33e723a5dd523fc145fc0"
TEST_SUBSCRIBER_SEQUENCE_NUMBER = "000000000022"

NUM_PROFILES = 5


class TestELLA:
    def test_given_ella_and_gnbsim_deployed_when_start_simulation_then_simulation_success_status_is_true(  # noqa: E501
        self,
    ):
        k8s_client = Kubernetes(namespace=GNBSIM_NAMESPACE)
        k8s_client.create_namespace()
        manifests = [
            "k8s/router-n6-nad.yaml",
            "k8s/router-deployment.yaml",
            "k8s/core-n2-nad.yaml",
            "k8s/core-n3-nad.yaml",
            "k8s/core-n6-nad.yaml",
            "k8s/core-configmap.yaml",
            "k8s/core-deployment.yaml",
            "k8s/core-service.yaml",
        ]
        for manifest in manifests:
            k8s_client.apply_manifest(manifest)
            logger.info(f"Applied manifest: {manifest}")
        logger.info("Waiting for Ella Core and Router components to be ready...")
        k8s_client.wait_for_app_ready(app_name="ella-core")
        k8s_client.wait_for_app_ready(app_name="router")
        logger.info("Ella Core and Router components are ready.")
        time.sleep(2)
        core_port = k8s_client.get_core_node_port(service_name="ella-core")
        core_address = f"http://127.0.0.1:{core_port}"
        subscriber = configure_ella_core(core_address=core_address)
        create_gnbsim_configmap(k8s_client, subscriber)
        gnbsim_manifests = [
            "k8s/gnbsim-n2-nad.yaml",
            "k8s/gnbsim-n3-nad.yaml",
            "k8s/gnbsim-deployment.yaml",
            "k8s/gnbsim-service.yaml",
        ]
        for manifest in gnbsim_manifests:
            k8s_client.apply_manifest(manifest)
            logger.info("Applied GNBSim manifest.")
        k8s_client.wait_for_app_ready(app_name="gnbsim")
        time.sleep(2)
        pod_name = k8s_client.get_pod_name(app_name="gnbsim")
        logger.info(f"Running GNBSim simulation in pod {pod_name}")
        result = k8s_client.exec(
            pod_name=pod_name,
            command="pebble exec gnbsim --cfg /etc/gnbsim/configuration.yaml",
            timeout=6 * 60,
        )
        logger.info(result)
        assert result.count("Profile Status: PASS") == NUM_PROFILES
        uplink_bytes, downlink_bytes = get_byte_count(core_address)
        assert uplink_bytes == 9000
        assert downlink_bytes == 9000
        k8s_client.delete_namespace()

    def test_given_ella_core_and_ueransim_deployed_when_start_simulation_then_simulation_success_status_is_true(  # noqa: E501
        self,
    ):
        k8s_client = Kubernetes(namespace=UERANSIM_NAMESPACE)
        k8s_client.create_namespace()
        manifests = [
            "k8s/router-n6-nad.yaml",
            "k8s/router-deployment.yaml",
            "k8s/core-n2-nad.yaml",
            "k8s/core-n3-nad.yaml",
            "k8s/core-n6-nad.yaml",
            "k8s/core-configmap.yaml",
            "k8s/core-deployment.yaml",
            "k8s/core-service.yaml",
            "k8s/ueransim-configmap.yaml",
            "k8s/ueransim-deployment.yaml",
            "k8s/ueransim-n2-nad.yaml",
            "k8s/ueransim-n3-nad.yaml",
        ]
        for manifest in manifests:
            k8s_client.apply_manifest(manifest)
            logger.info(f"Applied manifest: {manifest}")
        logger.info("Waiting for Ella Core, Router, and UERANSIM to be ready...")
        k8s_client.wait_for_app_ready(app_name="ella-core")
        k8s_client.wait_for_app_ready(app_name="router")
        k8s_client.wait_for_app_ready(app_name="ueransim")
        logger.info("Ella Core and Router components are ready.")
        time.sleep(2)
        core_port = k8s_client.get_core_node_port(service_name="ella-core")
        core_address = f"http://127.0.0.1:{core_port}"
        subscriber = configure_ella_core(core_address=core_address)
        time.sleep(2)
        patch_ue_configmap(k8s_client, subscriber)
        ueransim_pod_name = k8s_client.get_pod_name(app_name="ueransim")
        k8s_client.exec(
            pod_name=ueransim_pod_name, command="pebble add gnb /etc/ueransim/pebble_gnb.yaml"
        )
        k8s_client.exec(pod_name=ueransim_pod_name, command="pebble start gnb")
        logger.info(f"Started gnb pebble service in pod {ueransim_pod_name}")
        time.sleep(2)
        k8s_client.exec(
            pod_name=ueransim_pod_name, command="pebble add ue /etc/ueransim/pebble_ue.yaml"
        )
        k8s_client.exec(pod_name=ueransim_pod_name, command="pebble start ue")
        logger.info(f"Started ue pebble service in pod {ueransim_pod_name}")
        time.sleep(2)
        result = k8s_client.exec(
            pod_name=ueransim_pod_name,
            command="ip a",
            timeout=1 * 60,
        )
        logger.info(result)
        assert "uesimtun0" in result
        result = k8s_client.exec(
            pod_name=ueransim_pod_name,
            command="ping -I uesimtun0 192.168.250.1 -c 3",
            timeout=1 * 60,
        )
        logger.info(result)
        assert "3 packets transmitted, 3 received" in result
        assert "0% packet loss" in result
        k8s_client.delete_namespace()


def compute_imsi(base_imsi: str, increment: int) -> str:
    """Compute a new IMSI by incrementing the base IMSI.

    Args:
        base_imsi (str): The base IMSI as a string.
        increment (int): The number to increment the IMSI by.

    Returns:
        str: The new IMSI as a zero-padded string.
    """
    new_imsi = int(base_imsi) + increment
    return f"{new_imsi:015}"


def configure_ella_core(core_address: str) -> Subscriber:
    """Configure Ella Core.

    Returns:
        Subscriber: The first subscriber created in Ella Core.
    """
    ella_client = EllaCore(url=core_address)
    ella_client.create_user(email="admin@ellanetworks.com", password="admin", role="admin")
    token = ella_client.login(email="admin@ellanetworks.com", password="admin")
    if not token:
        raise RuntimeError("Failed to login to Ella Core")
    ella_client.set_token(token)
    ella_client.create_profile(
        name=TEST_PROFILE_NAME,
        dnn="internet",
        ue_ip_pool="172.250.0.0/24",
        dns="8.8.8.8",
        mtu=1460,
        bitrate_uplink="200 Mbps",
        bitrate_downlink="100 Mbps",
        priority_level=1,
        var5qi=8,
    )
    ella_client.update_operator_id(
        mcc="001",
        mnc="01",
    )
    ella_client.update_operator_slice(
        sst=1,
        sd=1056816,
    )
    ella_client.update_operator_tracking(
        supported_tacs=["001"],
    )
    for i in range(NUM_IMSIS):
        imsi = compute_imsi(TEST_START_IMSI, i)
        ella_client.create_subscriber(
            imsi=imsi,
            key=TEST_SUBSCRIBER_KEY,
            sequence_number=TEST_SUBSCRIBER_SEQUENCE_NUMBER,
            profile_name=TEST_PROFILE_NAME,
        )
    subscriber_0 = ella_client.get_subscriber(imsi=TEST_START_IMSI)
    return subscriber_0


def get_byte_count(core_address: str) -> Tuple[int, int]:
    """Get the uplink and downlink byte counts from Ella Core."""
    ella_client = EllaCore(url=core_address)
    token = ella_client.login(email="admin@ellanetworks.com", password="admin")
    if not token:
        raise RuntimeError("Failed to login to Ella Core")
    ella_client.set_token(token)
    uplink_bytes = ella_client.get_uplink_bytes_metric()
    downlink_bytes = ella_client.get_downlink_bytes_metric()
    return uplink_bytes, downlink_bytes


def create_gnbsim_configmap(k8s_client: Kubernetes, subscriber: Subscriber) -> None:
    """Generate and create the GNBSim ConfigMap with the subscriber information."""
    config = {
        "configuration.yaml": yaml.dump(
            {
                "configuration": {
                    "execInParallel": False,
                    "gnbs": {
                        "gnb1": {
                            "defaultAmf": {
                                "hostName": "192.168.253.3",
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
                            "n3IpAddr": "192.168.252.5",
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
                                },
                                {
                                    "broadcastPlmnList": [
                                        {
                                            "plmnId": {
                                                "mcc": "123",
                                                "mnc": "12",
                                            },
                                            "taiSliceSupportList": [
                                                {
                                                    "sd": "102031",
                                                    "sst": 1,
                                                }
                                            ],
                                        }
                                    ],
                                    "tac": "000002",
                                },
                            ],
                        },
                    },
                    "goProfile": {
                        "enable": False,
                        "port": 5005,
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
                            "ueCount": NUM_IMSIS,
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
                            "ueCount": NUM_IMSIS,
                        },
                        {
                            "profileType": "anrelease",
                            "profileName": "profile3",
                            "enable": True,
                            "gnbName": "gnb1",
                            "startImsi": subscriber.imsi,
                            "ueCount": NUM_IMSIS,
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
                            "ueCount": NUM_IMSIS,
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
                            "ueCount": NUM_IMSIS,
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
            },
            default_flow_style=False,
        )
    }

    configmap_manifest = {
        "apiVersion": "v1",
        "kind": "ConfigMap",
        "metadata": {
            "name": "gnbsim-config",
            "namespace": GNBSIM_NAMESPACE,
        },
        "data": config,
    }

    configmap_path = "/tmp/gnbsim-configmap.yaml"
    with open(configmap_path, "w") as f:
        yaml.dump(configmap_manifest, f)

    k8s_client.apply_manifest(configmap_path)
    logger.info("Created GNBSim ConfigMap.")


def patch_ue_configmap(k8s_client: Kubernetes, subscriber: Subscriber) -> None:
    """Patch the UERANSIM ConfigMap with the subscriber information.

    Get the existing ueransim-config ConfigMap, patch its ue.yaml data with the new values
    (IMSI, key, and OPC), and wait until the changes are reflected in the container.
    """
    configmap_name = "ueransim-config"
    logger.info(
        "Waiting for ConfigMap '%s' in namespace '%s'...", configmap_name, UERANSIM_NAMESPACE
    )
    timeout = 60
    start_time = time.time()
    configmap = k8s_client.get_configmap(configmap_name)
    logger.info("Found ConfigMap '%s'.", configmap_name)

    # Get the current ue.yaml content from the ConfigMap
    ue_yaml_str = configmap["data"].get("ue.yaml", "")
    if not ue_yaml_str:
        raise ValueError("The 'ue.yaml' key is missing in the ConfigMap data.")
    ue_config = yaml.safe_load(ue_yaml_str)

    # Update the necessary fields with values from the subscriber
    ue_config["supi"] = f"imsi-{subscriber.imsi}"
    ue_config["key"] = subscriber.key
    ue_config["op"] = subscriber.opc

    # Create the updated YAML string
    updated_ue_yaml_str = yaml.dump(ue_config, default_flow_style=False)

    # Construct a patch to update the 'ue.yaml' field
    patch = {"data": {"ue.yaml": updated_ue_yaml_str}}
    logger.info("Patching ConfigMap '%s' with updated subscriber values...", configmap_name)
    k8s_client.patch_configmap(configmap_name, patch)
    logger.info("Patched ConfigMap '%s'.", configmap_name)

    # Wait until the updated config is available in the container
    pod_name = k8s_client.get_pod_name("ueransim")
    logger.info("Waiting for updated config to be available in pod '%s'...", pod_name)
    timeout = 5*60
    start_time = time.time()
    while True:
        result = k8s_client.exec(
            pod_name=pod_name,
            command="cat /etc/ueransim/ue.yaml",
            timeout=10,
        )
        if subscriber.key in result and subscriber.opc in result:
            logger.info("Updated config is now available in container '%s'.", pod_name)
            break
        if time.time() - start_time > timeout:
            raise TimeoutError(
                "Updated config did not become available in container within timeout"
            )
        logger.info("Waiting for updated config to be available...")
        time.sleep(5)
