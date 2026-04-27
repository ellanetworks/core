# Subscriber Security

Info

To report a security vulnerability, please file a [Private Security Report](https://github.com/ellanetworks/core/security).

Ella Core implements **5G-AKA** (Authentication and Key Agreement) for secure, mutual authentication between the subscriber's device and the network.

The subscriber's Universal Subscriber Identity Module (USIM) stores the identity and credentials required for authentication:

- **IMSI (International Mobile Subscriber Identity)**: A globally unique identifier for the subscriber.
- **Key (Subscriber's Secret Key)**: A 128-bit cryptographic key shared between the USIM and the network.
- **OPc (Operator Code)**: A value derived from the operator key (OP) and the subscriber's secret key (K) using the Milenage algorithm.
- **SQN (Sequence Number)**: A counter maintained by both the USIM and the network to prevent replay attacks.

## Subscriber Privacy (SUCI)

Ella Core supports **SUCI** (Subscription Concealed Identifier) to protect subscriber identity over the air. The IMSI is encrypted by the subscriber's device before transmission using ECIES (Elliptic Curve Integrated Encryption Scheme). The network decrypts the SUCI to recover the SUPI. This prevents IMSI-catching attacks.

Two protection profiles are supported:

| Profile       | Curve                  | SUCI Scheme ID |
| ------------- | ---------------------- | -------------- |
| **Profile A** | Curve25519 (X25519)    | 1              |
| **Profile B** | NIST P-256 (secp256r1) | 2              |

Home network keys can be managed through the [Operator API](https://docs.ellanetworks.com/reference/api/operator/index.md) or the Operator page in the UI.

## NAS Security

After authentication, the network and the subscriber's device negotiate ciphering and integrity algorithms. Once established, these algorithms protect **all NAS signaling** for the lifetime of the connection.

Ella Core supports three ciphering algorithms (NEA0, NEA1/SNOW 3G, NEA2/AES) and three integrity algorithms (NIA0, NIA1/SNOW 3G, NIA2/AES). Administrators can configure which algorithms are enabled and their priority order through the [Operator API](https://docs.ellanetworks.com/reference/api/operator/index.md) or the Operator page in the UI.

Warning

Null algorithms (NEA0/NIA0) provide no security protection. Only enable them for testing or device compatibility.

## Managing Subscriber Credentials

Users can update the Operator Key (OP) via the [Operator API](https://docs.ellanetworks.com/reference/api/operator/index.md) or the UI.

When creating a new subscriber via the [Subscribers API](https://docs.ellanetworks.com/reference/api/subscribers/index.md) or the UI, Ella Core automatically computes the OPc using the provided Key and the current OP value.

The UI provides a user-friendly interface for automatically generating IMSIs, Keys, and SQNs for new subscribers.
