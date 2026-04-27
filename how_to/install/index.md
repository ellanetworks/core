# Install

Ensure your system meets the [requirements](https://docs.ellanetworks.com/reference/system_reqs/index.md). Then, choose one of the installation methods below.

Install the Ella Core snap and connect it to the required interfaces:

```
sudo snap install ella-core
sudo snap connect ella-core:network-control
sudo snap connect ella-core:process-control
sudo snap connect ella-core:system-observe
sudo snap connect ella-core:firewall-control
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
sudo apt -y install clang llvm gcc-multilib libbpf-dev
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
        path: "data"
      interfaces:
        n2:
          address: "10.3.0.2"
          port: 38412
        n3:
          name: "n3"
        n6:
          name: "eth0"
        api:
          address: "0.0.0.0"
          port: 5002
      xdp:
        attach-mode: "generic"

services:
  ella-core:
    image: ghcr.io/ellanetworks/ella-core:v1.9.1
    configs:
      - source: ella_config
        target: /core.yaml
    restart: unless-stopped
    entrypoint: /bin/core --config /core.yaml
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

```
docker compose up -d
```

Ensure your Kubernetes cluster is running with the [Multus CNI](https://github.com/k8snetworkplumbingwg/multus-cni) installed.

```
kubectl apply -k github.com/ellanetworks/core/k8s?ref=v1.9.1 -n ella
```
