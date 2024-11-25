#!/usr/bin/env python3
# Copyright 2024 Guillaume Belanger
# See LICENSE file for licensing details.

"""Module use to handle Ella API calls."""

import json
import logging
from typing import Any, List, Optional

import requests

logger = logging.getLogger(__name__)


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
        "ue-ip-pool": "172.250.1.0/16",
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
        "gNodeBs": [{"name": "demo-gnb1", "tac": 1}],
        "upf": {"upf-name": "upf-external", "upf-port": "8805"},
    },
}


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
        data: any = None,
    ) -> Any | None:
        """Make an HTTP request and handle common error patterns."""
        headers = JSON_HEADER
        url = f"{self.url}{endpoint}"
        logger.info("%s request to %s", method, url)
        try:
            response = requests.request(
                method=method,
                url=url,
                headers=headers,
                json=data,
                verify=False,
            )
        except requests.exceptions.SSLError as e:
            logger.error("SSL error: %s", e)
            return None
        except requests.RequestException as e:
            logger.error("HTTP request failed: %s", e)
            return None
        except OSError as e:
            logger.error("couldn't complete HTTP request: %s", e)
            return None
        try:
            response.raise_for_status()
        except requests.HTTPError:
            logger.error(
                "Request failed: code %s",
                response.status_code,
            )
            return None
        try:
            json_response = response.json()
        except json.JSONDecodeError:
            return None
        return json_response

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
