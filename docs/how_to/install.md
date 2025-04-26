---
description: Step-by-step instructions to install Ella Core.
---

# Install

You can install Ella Core on Linux or on Kubernetes.

=== "Linux"

    ## Using the Snap (Recommended)

    Ella Core is available as a Snap, making it easy to install on many Linux distributions, including Ubuntu, Ubuntu Core, Debian, Arch Linux, and more. View the Ella Core snap on the [Snap Store](https://snapcraft.io/ella-core).

    ### Pre-requisites

    - A machine with at least:
        - 2 CPU cores
        - 2 GB of RAM
        - 10 GB of disk space
        - 4 network interfaces
    - A Linux distribution that supports Snap packages.
  
    ### Steps

    Install the snap:

    ```bash
    sudo snap install ella-core
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
    logging:
      system:
        level: "info"
        output: "stdout"
      audit:
        output: "stdout"
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

    ## Building from Source

    You can build Ella Core from source.

    !!! warning
        Building from source is recommended for development purposes only.

    ### Pre-requisites

    Install the pre-requisites:

    ```shell
    sudo snap install go --channel=1.24/stable --classic
    sudo snap install node --channel=20/stable --classic
    sudo apt update
    sudo apt -y install clang llvm gcc-multilib libbpf-dev
    ```

    ### Steps

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
        For example, to build version 0.0.17, run `git checkout v0.0.17`.

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
    sudo ./main --config core.yaml
    ```

    Navigate to `https://localhost:5002` to access the Ella UI.

=== "Kubernetes"

    Ella Core is available as a Container image, making it easy to deploy on Kubernetes. View the Ella Core image on the [GitHub Container Registry](https://github.com/ellanetworks/core/pkgs/container/ella-core).

    ### Pre-requisites
    - A Kubernetes cluster with:
        - Multus CNI installed

    ### Steps

    Create a namespace for Ella Core:

    ```bash
    kubectl create namespace ella
    ```

    Install Ella Core:
    ```bash
    kubectl apply -k github.com/ellanetworks/core/k8s/core/base?ref=v0.0.17 -n ella
    ```

    !!! note
        You can change the configuration by editing the `core-config` ConfigMap:

        ```bash
        kubectl edit configmap core-config -n ella
        ```
