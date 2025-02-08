---
description: Step-by-step instructions to deploy Ella Core.
---

# Deploy

Ella Core is available as a Snap and a container image.

=== "Snap (Recommended)"

    Ella Core is available as a Snap. View it on the [Snap Store](https://snapcraft.io/ella-core).

    ## Pre-requisites

    - A machine with:
        - 2 CPU cores
        - 4 GB of RAM
        - 10 GB of disk space
        - 3 network interfaces
    - A Linux distribution that supports Snap packages.
  
    ## Steps

    Ella Core is available as a snap package.

    Install the snap:

    ```bash
    sudo snap install ella-core --channel=beta
    ```

    Connect the snap to the required interfaces:

    ```bash
    sudo snap connect ella-core:network-control
    sudo snap connect ella-core:process-control
    sudo snap connect ella-core:sys-fs-bpf-upf-pipeline
    sudo snap connect ella-core:system-observe
    ```

    Edit the configuration file at `/var/snap/ella-core/common/config.yaml` to configure the network interfaces:

    ```yaml
    log-level: "info"
    db:
      path: "/var/snap/ella-core/common/data/core.db"
    interfaces:
      n2:
        name: "ens4"
        port: 38412
      n3: 
        name: "ens5"
      n6:
        name: "ens6"
      api:
        name: "ens3"
        port: 5002
        tls:
          cert: "/var/snap/ella-core/common/cert.pem"
          key: "/var/snap/ella-core/common/key.pem"
    xdp:
        attach-mode: "native"
    ```

    !!! note
        
        For more information on the configuration options, see the [configuration file reference](../reference/config_file.md).

    Start the service:
    ```bash
    sudo snap start ella-core.cored
    ```

    Navigate to `https://localhost:5002` to access the Ella UI.


=== "Container"

    We provide a container image for Ella Core on GitHub Container Registry.

    Pull the image from the registry:

    ```shell
    docker pull ghcr.io/ellanetworks/ella-core:latest
    ```

    Installation can then be done using the approach of your choice. 

=== "Source"

    You can build Ella Core from source.

    !!! warning
        Building from source is recommended for development purposes only.

    ## Pre-requisites

    Install the pre-requisites:

    ```shell
    sudo snap install go --channel=1.23/stable --classic
    sudo snap install node --channel=20/stable --classic
    sudo apt install clang llvm gcc-multilib libbpf-dev
    ```

    Clone the Ella Core repository:

    ```shell
    git clone https://github.com/ellanetworks/core.git
    ```

    Move to the repository directory:

    ```shell
    cd core
    ```

    !!! note
        If you want to build a specific version, checkout the tag after cloning the repository.
        For example, to build version 0.0.4, run `git checkout v0.0.4`.

    Build the frontend:
    ```shell
    npm install --prefix ui
    npm run build --prefix ui
    ```

    Build the backend:
    ```shell
    go build cmd/core/main.go
    ```

    Edit the configuration file at `core.yaml` to configure the network interfaces.

    Start the service:
    ```shell
    ./main --config core.yaml
    ```

    Navigate to `https://localhost:5002` to access the Ella UI.
