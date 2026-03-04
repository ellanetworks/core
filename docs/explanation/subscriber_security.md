---
description: Explanation of subscriber security - authentication, privacy, and NAS protection.
---

# Subscriber Security

!!! info
    To report a security vulnerability, please file a [Private Security Report](https://github.com/ellanetworks/core/security).

Ella Core implements **5G-AKA** (Authentication and Key Agreement) for secure, mutual authentication between the subscriber's device and the network.

The subscriber's Universal Subscriber Identity Module (USIM) stores the identity and credentials required for authentication:

- **IMSI (International Mobile Subscriber Identity)**: A globally unique identifier for the subscriber.
- **Key (Subscriber's Secret Key)**: A 128-bit cryptographic key shared between the USIM and the network.
- **OPc (Operator Code)**: A value derived from the operator key (OP) and the subscriber's secret key (K) using the Milenage algorithm (see [3GPP TS 35.206](https://www.3gpp.org/DynaReport/35206.htm)).
- **SQN (Sequence Number)**: A counter maintained by both the USIM and the network to prevent replay attacks.

## Algorithms

Ella Core uses the **Milenage** algorithm set built on AES-128 to produce authentication vectors, as defined in [3GPP TS 35.206](https://www.3gpp.org/DynaReport/35206.htm). Key derivation follows [3GPP TS 33.501](https://www.3gpp.org/DynaReport/33501.htm).

## Subscriber Privacy (SUCI)

Ella Core supports **SUCI** (Subscription Concealed Identifier) to protect subscriber identity over the air. The IMSI is encrypted by the UE before transmission using **ECIES Profile A**:

- **X25519** for key agreement
- **AES-128-CTR** for encryption
- **HMAC-SHA-256** for integrity

The network decrypts the SUCI to recover the SUPI. This prevents IMSI-catching attacks.

## NAS Security

After authentication, NAS signaling between the UE and the network is protected with ciphering and integrity algorithms. Ella Core supports the following algorithms:

| Type | Algorithms |
|------|------------|
| **Ciphering** | NEA0, NEA1 (SNOW 3G), NEA2 (AES) |
| **Integrity** | NIA0, NIA1 (SNOW 3G), NIA2 (AES) |

## Managing Subscriber Credentials

Users can update the Operator Key (OP) via the [Operator API](../reference/api/operator.md) or the UI.

When creating a new subscriber via the [Subscribers API](../reference/api/subscribers.md) or the UI, Ella Core automatically computes the OPc using the provided Key and the current OP value.

The UI provides a user-friendly interface for automatically generating IMSIs, Keys, and SQNs for new subscribers.
