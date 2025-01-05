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

    Edit the configuration file at `/var/snap/ella-core/common/config.yaml` to configure the network interfaces:

    ```yaml
    log-level: "info"
    db:
    path: "core.db"
    interfaces: 
    n3: 
        name: "ens4"
        address: "127.0.0.1"
    n6:
        name: "ens5"
    api:
        name: "lo"
        port: 5002
        tls:
        cert: "/var/snap/ella-core/common/cert.pem"
        key: "/var/snap/ella-core/common/key.pem"
    ```

    !!! note
        
        For more information on the configuration options, see the [configuration file reference](../reference/config_file.md).

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
