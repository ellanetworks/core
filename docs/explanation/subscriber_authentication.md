---
description: Explanation of subscriber authentication - IMSI, Key, OP, OPc, and SQN.
---

# Subscriber Authentication

## Overview

Subscriber authentication in 5G networks can be based on one of the following mechanisms:

- 5G-AKA
- EAP-AKA

These protocol ensure secure and mutual authentication between the subscriber's device and the network, establishing a secure channel for communication.

The subscriber's Universal Subscriber Identity Module (USIM) stores critical information required for authentication, including:

- **IMSI (International Mobile Subscriber Identity)**: A globally unique identifier for the subscriber, typically represented as a string of decimal digits.
- **Key (Subscriber's Secret Key)**: A cryptographic key shared between the USIM and the Private Network for authentication and encryption.
- **OPc (Operator Code)**: A derived value computed as OPc = AES-128(Key, OP), resulting in user specific operator code.
- **SQN (Sequence Number)**: A counter maintained by both the USIM and the network to prevent replay attacks.

## Subscriber Authentication in Ella Core

Ella Core implements both the 5G-AKA and EAP-AKA mechanisms as part of its subscriber authentication process. 

Users can update the Operator Key (OP) via the [Operator API](../reference/api/operator.md) or the UI.

When creating a new subscriber via the [Subscribers API](../reference/api/subscribers.md) or the UI, Ella Core automatically computes the OPc using the provided Key (subscriber key) and the current OP value.

The UI provides a user-friendly interface for automatically generating IMSI's, Key's, and SQN's for new subscribers.
