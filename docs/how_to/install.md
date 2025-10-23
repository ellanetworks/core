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
    go build cmd/core/main.go
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

    Create a Docker network for the n3 interface:

    ```shell
    docker network create --driver bridge n3 --subnet 10.3.0.0/24
    ```

    Start the Ella Core container with the additional network interfaces:

    ```shell
    docker create \
    --name ella-core \
    --privileged \
    --network=name=bridge \
    -p 5002:5002 \
    -v /sys/fs/bpf:/sys/fs/bpf:rw \
    ella-core:latest exec /bin/core --config /core.yaml
    docker network connect --driver-opt com.docker.network.endpoint.ifname=n3 --ip 10.3.0.2 n3 ella-core
    docker start ella-core
    ```

=== "Kubernetes"

    Ensure your Kubernetes cluster is running with the [Multus CNI](https://github.com/k8snetworkplumbingwg/multus-cni) installed.

    ```bash
    kubectl apply -k github.com/ellanetworks/core/k8s?ref=v0.4.0 -n ella
    ```
