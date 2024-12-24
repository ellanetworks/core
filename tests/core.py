#!/usr/bin/env python3
# Copyright 2024 Ella Networks
# See LICENSE file for licensing details.

"""Module use to handle Ella API calls."""

import logging
from dataclasses import asdict, dataclass
from typing import Any

import requests

logger = logging.getLogger(__name__)

GNB_CONFIG_URL = "/api/v1/radios"
NETWORK_CONFIG_URL = "/api/v1/network"
PROFILE_CONFIG_URL = "/api/v1/profiles"
SUBSCRIBERS_CONFIG_URL = "/api/v1/subscribers"

JSON_HEADER = {"Content-Type": "application/json"}

SUBSCRIBER_CONFIG = {
    "imsi": "PLACEHOLDER",
    "opc": "981d464c7c52eb6e5036234984ad0bcf",
    "key": "5122250214c33e723a5dd523fc145fc0",
    "sequenceNumber": "16f3b3f70fc2",
    "profileName": "PLACEHOLDER",
}

PROFILE_CONFIG = {
    "name": "PLACEHOLDER",
    "dnn": "internet",
    "ue-ip-pool": "172.250.0.0/16",
    "dns": "8.8.8.8",
    "mtu": 1460,
    "bitrate-uplink": "200 Mbps",
    "bitrate-downlink": "100 Mbps",
    "priority-level": 1,
    "var5qi": 8
}


NETWORK_CONFIG = {
    "mcc": "001",
    "mnc": "01",
}


@dataclass
class CreateRadioParams:
    """Parameters to create a radio."""
    name: str
    tac: str


class EllaCore:
    """Handle Ella Core API calls."""

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

    def create_radio(self, name: str, tac: str) -> None:
        """Create a radio in the NMS."""
        create_radio_params = CreateRadioParams(name=name, tac=str(tac))
        self._make_request("POST", GNB_CONFIG_URL, data=asdict(create_radio_params))
        logger.info("Radio %s created in NMS", name)

    def create_subscriber(self, imsi: str, profile_name: str) -> None:
        """Create a subscriber."""
        data = SUBSCRIBER_CONFIG.copy()
        data["imsi"] = imsi
        data["profileName"] = profile_name
        self._make_request(method="POST", endpoint=SUBSCRIBERS_CONFIG_URL, data=data)
        logger.info(f"Created subscriber with IMSI {imsi}.")

    def create_profile(self, name: str) -> None:
        """Create a profile."""
        PROFILE_CONFIG["name"] = name
        self._make_request("POST", PROFILE_CONFIG_URL, data=PROFILE_CONFIG)
        logger.info(f"Created profile {name}.")

    def update_network(self) -> None:
        """Create a network slice."""
        self._make_request("PUT", NETWORK_CONFIG_URL, data=NETWORK_CONFIG)
        logger.info("Updated network configuration.")
