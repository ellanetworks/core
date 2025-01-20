---
description: Reference for TLS.
---

# TLS

Ella Core uses TLS to secure its API and web interface. The use of TLS is mandatory, Ella Core will not start if the TLS configuration is missing or invalid.

## Configuration

The TLS configuration is defined in the [configuration file](config_file.md). 

## Default Certificate

When installing the Ella Core snap, a self-signed certificate is generated and stored in the snap's common directory. The certificate is valid for 365 days. Users can replace the certificate and key at any time by updating the respective files in the common directory. 

## Considerations for Production

It is highly recommended to replace the self-signed certificate with one issued by a trusted Certificate Authority (CA) for production deployments.

Ensure that the private key is stored securely and that access is restricted to authorized personnel only.

## Certificate Renewal

After replacing the files, the Ella Core service must be restarted to apply the changes.

## Supported TLS Versions

Ella Core supports TLS versions `1.2` and `1.3`.
