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
OPERATOR_ID_CONFIG_URL = "/api/v1/operator/id"
PROFILE_CONFIG_URL = "/api/v1/profiles"
SUBSCRIBERS_CONFIG_URL = "/api/v1/subscribers"

JSON_HEADER = {"Content-Type": "application/json"}

SUBSCRIBER_CONFIG = {
    "imsi": "PLACEHOLDER",
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
    "var5qi": 8,
}

OPERATOR_ID_CONFIG = {
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
        self.token = None

    def _make_request(
        self,
        method: str,
        endpoint: str,
        data: any = None,  # type: ignore[reportGeneralTypeIssues]
    ) -> Any | None:
        """Make an HTTP request and handle common error patterns."""
        headers = JSON_HEADER
        if self.token:
            headers["Authorization"] = f"Bearer {self.token}"
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

    def set_token(self, token: str) -> None:
        """Set the authentication token."""
        self.token = token

    def login(self, username: str, password: str) -> str | None:
        """Login to Ella Core.

        Returns:
            str: The authentication token.
        """
        data = {"username": username, "password": password}
        response = self._make_request("POST", "/api/v1/login", data=data)
        result = response.get("result")
        if not result:
            logger.error("Failed to login to Ella Core.")
            return None
        token = result.get("token")
        if not token:
            logger.error("Failed to login to Ella Core.")
            return None
        logger.info("Logged in to Ella Core.")
        return token

    def create_user(self, username: str, password: str) -> None:
        """Create a user in Ella Core."""
        data = {"username": username, "password": password}
        self._make_request("POST", "/api/v1/users", data=data)
        logger.info("User %s created in Ella Core", username)

    def create_radio(self, name: str, tac: str) -> None:
        """Create a radio in Ella Core."""
        create_radio_params = CreateRadioParams(name=name, tac=str(tac))
        self._make_request("POST", GNB_CONFIG_URL, data=asdict(create_radio_params))
        logger.info("Radio %s created in Ella Core", name)

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

    def update_operator_id(self) -> None:
        """Update operator ID information."""
        self._make_request("PUT", OPERATOR_ID_CONFIG_URL, data=OPERATOR_ID_CONFIG)
        logger.info("Updated network configuration.")
