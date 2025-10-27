---
description: Step-by-step instructions to install Ella Core.
---

# Install

Ensure your system meets the [requirements](../reference/system_reqs.md). Then, choose one of the installation methods below.

=== "Snap (Recommended)"

    Install the Ella Core snap and connect it to the required interfaces:

    ```bash
    sudo snap install ella-core
    sudo snap connect ella-core:network-control
    sudo snap connect ella-core:process-control
    sudo snap connect ella-core:sys-fs-bpf-upf-pipeline
    sudo snap connect ella-core:system-observe
    sudo snap connect ella-core:firewall-control
    ```

    Configure Ella Core:

    ```bash
    sudo vim /var/snap/ella-core/common/core.yaml
    ```

    Start Ella Core:

    ```bash
    sudo snap start --enable ella-core.cored
    ```

=== "Source"

    Install the required dependencies:

    ```shell
    sudo snap install go --channel=1.24/stable --classic
    sudo snap install node --channel=22/stable --classic
    sudo apt update
    sudo apt -y install clang llvm gcc-multilib libbpf-dev
    ```

    Clone the Ella Core repository:

    ```shell
    git clone https://github.com/ellanetworks/core.git
    cd core
    ```

    Build the frontend:

    ```shell
    npm install --prefix ui
    npm run build --prefix ui
    ```

    Build Ella Core:

    ```shell
    REVISION=`git rev-parse HEAD`
    go build -ldflags "-X github.com/ellanetworks/core/version.GitCommit=${REVISION}" ./cmd/core/main.go
    ```

    Configure Ella Core:

    ```bash
    vim core.yaml
    ```

    Start Ella Core:

    ```bash
    sudo ./main -config core.yaml
    ```

=== "Docker"

    Create a new directory:

    ```shell
    mkdir ella
    cd ella
    ```

    Copy the following file into this directory:

    ```yaml title="docker-compose.yaml"
    configs:
      ella_config:
        content: |
          logging:
            system:
              level: "info"
              output: "stdout"
            audit:
              output: "stdout"
          db:
            path: "core.db"
          interfaces:
            n2:
              name: "n3"
              port: 38412
            n3:
              name: "n3"
            n6:
              name: "eth0"
            api:
              name: "eth0"
              port: 5002
          xdp:
            attach-mode: "generic"

    services:
      ella-core:
        image: ghcr.io/ellanetworks/ella-core:v0.4.0
        configs:
          - source: ella_config
            target: /core.yaml
        restart: unless-stopped
        entrypoint: /bin/core --config /core.yaml
        volumes:
          - /sys/fs/bpf:/sys/fs/bpf:rw
        privileged: true
        ports:
          - "5002:5002"
        networks:
          default:
            driver_opts:
              com.docker.network.endpoint.ifname: eth0
          n3:
            driver_opts:
              com.docker.network.endpoint.ifname: n3
            ipv4_address: 10.3.0.2

    networks:
      n3:
        internal: true
        ipam:
          config:
            - subnet: 10.3.0.0/24
    ```

    Edit the file to match your network interfaces and desired configuration.

    Start the Ella Core container:

    ```shell
    docker compose up -d
    ```

=== "Kubernetes"

    Ensure your Kubernetes cluster is running with the [Multus CNI](https://github.com/k8snetworkplumbingwg/multus-cni) installed.

    ```bash
    kubectl apply -k github.com/ellanetworks/core/k8s?ref=v0.4.0 -n ella
    ```
