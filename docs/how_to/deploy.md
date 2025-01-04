---
description: Step-by-step instructions to deploy Ella Core.
---

# Deploy

Ella Core is available as a Snap and a OCI container image.

=== "Snap (Recommended)"

    Ella Core is available as a snap package [here](https://snapcraft.io/ella-core).

    Install the snap:

    ```bash
    sudo snap install ella-core --channel=edge --devmode
    ```

    Generate (or copy) a certificate and private key to the following location:

    ```bash
    sudo openssl req -newkey rsa:2048 -nodes -keyout /var/snap/ella-core/common/key.pem -x509 -days 1 -out /var/snap/ella-core/common/cert.pem -subj "/CN=example.com"
    ```

    Start the service:
    ```bash
    sudo snap start ella-core.cored
    ```

    Navigate to `https://localhost:5000` to access the Ella UI.


=== "OCI Container Image"

    We provide a container image for Ella Core on GitHub Container Registry.

    Pull the image from the registry:

    ```shell
    docker pull ghcr.io/ellanetworks/ella-core:v0.0.4
    ```

    Installation can then be done using the approach of your choice. 

    !!! note
        We are planning on publishing a Juju Kubernetes charm in the future. 
        This charm will allow you to operate Ella Core on Kubernetes.
