#!/usr/bin/env python3
# Copyright 2024 Guillaume Belanger
# See LICENSE file for licensing details.

"""Module use to handle Ella API calls."""

import logging
from dataclasses import asdict, dataclass
from typing import Any, List

import requests

logger = logging.getLogger(__name__)

GNB_CONFIG_URL = "/api/v1/radios"
NETWORK_SLICE_CONFIG_URL = "/api/v1/network-slices"
DEVICE_GROUPS_CONFIG_URL = "/api/v1/profiles"
SUBSCRIBERS_CONFIG_URL = "/api/v1/subscribers"

JSON_HEADER = {"Content-Type": "application/json"}

SUBSCRIBER_CONFIG = {
    "UeId": "PLACEHOLDER",
    "plmnId": "00101",
    "opc": "981d464c7c52eb6e5036234984ad0bcf",
    "key": "5122250214c33e723a5dd523fc145fc0",
    "sequenceNumber": "16f3b3f70fc2",
}

DEVICE_GROUP_CONFIG = {
    "imsis": [],
    "site-info": "demo",
    "ip-domain-name": "pool1",
    "ip-domain-expanded": {
        "dnn": "internet",
        "ue-ip-pool": "172.250.0.0/16",
        "dns-primary": "8.8.8.8",
        "mtu": 1460,
        "ue-dnn-qos": {
            "dnn-mbr-uplink": 200000000,
            "dnn-mbr-downlink": 200000000,
            "bitrate-unit": "bps",
            "traffic-class": {"name": "platinum", "arp": 6, "pdb": 300, "pelr": 6, "qci": 8},
        },
    },
}


NETWORK_SLICE_CONFIG = {
    "slice-id": {"sst": "1", "sd": "102030"},
    "site-device-group": [],
    "site-info": {
        "site-name": "demo",
        "plmn": {"mcc": "001", "mnc": "01"},
        "gNodeBs": [{"name": "dev2-gnbsim", "tac": 1}],
        "upf": {"upf-name": "0.0.0.0", "upf-port": "8806"},
    },
}


@dataclass
class CreateRadioParams:
    """Parameters to create a radio."""
    name: str
    tac: str


class Ella:
    """Handle Ella API calls."""

    def __init__(self, url: str):
        if url.endswith("/"):
            url = url[:-1]
        self.url = url

    def _make_request(
        self,
        method: str,
        endpoint: str,
        data: any = None,  # type: ignore[reportGeneralTypeIssues]
    ) -> Any | None:
        """Make an HTTP request and handle common error patterns."""
        headers = JSON_HEADER
        url = f"{self.url}{endpoint}"
        logger.info("%s request to %s", method, url)
        response = requests.request(
            method=method,
            url=url,
            headers=headers,
            json=data,
            verify=False,
        )
        response.raise_for_status()
        json_response = response.json()
        return json_response

    def create_radio(self, name: str, tac: int) -> None:
        """Create a radio in the NMS."""
        create_radio_params = CreateRadioParams(name=name, tac=str(tac))
        self._make_request("POST", GNB_CONFIG_URL, data=asdict(create_radio_params))
        logger.info("Radio %s created in NMS", name)

    def create_subscriber(self, imsi: str) -> None:
        """Create a subscriber."""
        data = SUBSCRIBER_CONFIG.copy()
        data["UeId"] = imsi
        self._make_request(method="POST", endpoint=SUBSCRIBERS_CONFIG_URL, data=data)
        logger.info(f"Created subscriber with IMSI {imsi}.")

    def create_device_group(self, name: str, imsis: List[str]) -> None:
        """Create a device group."""
        DEVICE_GROUP_CONFIG["imsis"] = imsis
        DEVICE_GROUP_CONFIG["name"] = name
        self._make_request("POST", DEVICE_GROUPS_CONFIG_URL, data=DEVICE_GROUP_CONFIG)
        logger.info(f"Created device group {name}.")

    def create_network_slice(self, name: str, device_groups: List[str]) -> None:
        """Create a network slice."""
        NETWORK_SLICE_CONFIG["site-device-group"] = device_groups
        NETWORK_SLICE_CONFIG["slice-name"] = name
        self._make_request("POST", NETWORK_SLICE_CONFIG_URL, data=NETWORK_SLICE_CONFIG)
        logger.info(f"Created network slice {name}.")
