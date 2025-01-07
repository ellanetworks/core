#!/usr/bin/env python3

from tests.core import EllaCore

TEST_PROFILE_NAME = "default-default"
TEST_START_IMSI = "001010100000001"
NUM_IMSIS = 200
TEST_SUBSCRIBER_KEY = "5122250214c33e723a5dd523fc145fc0"
TEST_SUBSCRIBER_SEQUENCE_NUMBER = "000000000001"


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


def main():
    core_address = "https://127.0.0.1:5002"
    ella_client = EllaCore(url=core_address)
    token = ella_client.login(email="admin@ellanetworks.com", password="admin")
    if not token:
        raise RuntimeError("Failed to login to Ella Core")
    ella_client.set_token(token)
    for i in range(NUM_IMSIS):
        imsi = compute_imsi(TEST_START_IMSI, i)
        ella_client.create_subscriber(
            imsi=imsi,
            key=TEST_SUBSCRIBER_KEY,
            sequence_number=TEST_SUBSCRIBER_SEQUENCE_NUMBER,
            profile_name=TEST_PROFILE_NAME,
        )


if __name__ == "__main__":
    main()
