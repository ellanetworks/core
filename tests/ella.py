#!/usr/bin/env python3
# Copyright 2024 Guillaume Belanger
# See LICENSE file for licensing details.

"""Module use to handle Ella API calls."""

import logging
from dataclasses import asdict, dataclass
from typing import Any, List

import requests

logger = logging.getLogger(__name__)

GNB_CONFIG_URL = "config/v1/inventory/gnb"

JSON_HEADER = {"Content-Type": "application/json"}

SUBSCRIBER_CONFIG = {
    "UeId": "PLACEHOLDER",
    "plmnId": "00101",
    "opc": "981d464c7c52eb6e5036234984ad0bcf",
    "key": "5122250214c33e723a5dd523fc145fc0",
    "sequenceNumber": "16f3b3f70fc2",
}

DEVICE_GROUP_CONFIG = {
    "group-name": "default-default",
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
    "slice-name": "default",
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
class CreateGnbParams:
    """Parameters to create a gNB."""

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

    def create_gnb(self, name: str, tac: int) -> None:
        """Create a gNB in the NMS inventory."""
        create_gnb_params = CreateGnbParams(tac=str(tac))
        self._make_request("POST", f"/{GNB_CONFIG_URL}/{name}", data=asdict(create_gnb_params))
        logger.info("gNB %s created in NMS", name)

    def create_subscriber(self, imsi: str) -> None:
        """Create a subscriber."""
        url = f"/api/subscriber/imsi-{imsi}"
        data = SUBSCRIBER_CONFIG.copy()
        data["UeId"] = imsi
        self._make_request(method="POST", endpoint=url, data=data)
        logger.info(f"Created subscriber with IMSI {imsi}.")

    def create_device_group(self, name: str, imsis: List[str]) -> None:
        """Create a device group."""
        DEVICE_GROUP_CONFIG["imsis"] = imsis
        url = f"/config/v1/device-group/{name}"
        self._make_request("POST", url, data=DEVICE_GROUP_CONFIG)
        logger.info(f"Created device group {name}.")

    def create_network_slice(self, name: str, device_groups: List[str]) -> None:
        """Create a network slice."""
        NETWORK_SLICE_CONFIG["site-device-group"] = device_groups
        url = f"/config/v1/network-slice/{name}"
        self._make_request("POST", url, data=NETWORK_SLICE_CONFIG)
        logger.info(f"Created network slice {name}.")
