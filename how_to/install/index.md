# Install

Ensure your system meets the [requirements](https://docs.ellanetworks.com/reference/system_reqs/index.md). Then, choose one of the installation methods below.

Install the Ella Core snap and connect it to the required interfaces:

```
sudo snap install ella-core
sudo snap connect ella-core:network-control
sudo snap connect ella-core:process-control
sudo snap connect ella-core:firewall-control
sudo snap connect ella-core:mount-observe
```

Configure Ella Core:

```
sudo vim /var/snap/ella-core/common/core.yaml
```

Start Ella Core:

```
sudo snap start --enable ella-core.cored
```

Install the required dependencies:

```
sudo snap install go --channel=1.26/stable --classic
sudo snap install node --channel=24/stable --classic
sudo apt update
sudo apt -y install gcc
```

Clone the Ella Core repository:

```
git clone https://github.com/ellanetworks/core.git
cd core
```

Build the frontend:

```
npm install --prefix ui
npm run build --prefix ui
```

Build Ella Core:

```
REVISION=`git rev-parse HEAD`
go build -ldflags "-X github.com/ellanetworks/core/version.GitCommit=${REVISION}" ./cmd/core/main.go
```

Configure Ella Core:

```
vim core.yaml
```

Start Ella Core:

```
sudo ./main -config core.yaml
```

Create a new directory:

```
mkdir ella
cd ella
```

Copy the following file into this directory:

docker-compose.yaml

```
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
        path: "/data/ella.db"
      interfaces:
        n2:
          address: "10.3.0.2"
          ngap-port: 38412
        n3:
          name: "n3"
        n6:
          name: "eth0"
        api:
          address: "0.0.0.0"
          port: 5002
      datapath:
        attach-mode: "tcx"

services:
  ella-core:
    image: ghcr.io/ellanetworks/ella-core:v1.19.0
    configs:
      - source: ella_config
        target: /core.yaml
    restart: unless-stopped
    entrypoint: /bin/core --config /core.yaml
    privileged: true
    volumes:
      - ella-data:/data
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

volumes:
  ella-data:
```

Edit the file to match your network interfaces and desired configuration.

Note

This example uses `tcx` mode. To use `xdp-native`, run the container with `network_mode: host` and set `n3` and `n6` to host NICs with native XDP support.

Start the Ella Core container:

```
docker compose up -d
```

Ensure your Kubernetes cluster is running with the [Multus CNI](https://github.com/k8snetworkplumbingwg/multus-cni) installed.

Download the Ella Core manifests:

```
git clone --depth 1 --branch v1.19.0 https://github.com/ellanetworks/core.git
```

Edit `core/k8s/core-ran-nad.yaml` to set `master` to the host interface that connects to your radios, and set the N2/N3 address. Use the same address for `interfaces.n2.address` in `core/k8s/core-configmap.yaml`.

Deploy Ella Core:

```
kubectl apply -k core/k8s
```

Note

These manifests use `tcx` mode. To use `xdp-native`, move physical N3 and N6 NICs with native XDP support into the pod (e.g. with `host-device`) instead of using macvlan and `eth0`, and put both in a VRF.
