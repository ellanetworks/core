---
description: Step-by-step instructions to install Ella Core.
---

# Install


=== "Linux"

    Ensure your system meets the [requirements](../reference/system_reqs.md).

    ## Using Snap (Recommended)

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

    ## From Source (For Development)

    ```shell
    sudo snap install go --channel=1.24/stable --classic
    sudo snap install node --channel=22/stable --classic
    sudo apt update
    sudo apt -y install clang llvm gcc-multilib libbpf-dev
    git clone https://github.com/ellanetworks/core.git
    cd core
    npm install --prefix ui
    npm run build --prefix ui
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

=== "Kubernetes"

    Ensure your Kubernetes cluster is running with the [Multus CNI](https://github.com/k8snetworkplumbingwg/multus-cni) installed.

    ```bash
    kubectl apply -k github.com/ellanetworks/core/k8s/core/base?ref=v0.3.1 -n ella
    ```
