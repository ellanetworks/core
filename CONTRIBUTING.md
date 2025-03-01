# Contributing

## Getting Started

### Set up MicroK8s

```shell
sudo snap install microk8s --channel=1.32/stable --classic
```

Enable the required addons:
```shell
sudo microk8s addons repo add community https://github.com/canonical/microk8s-community-addons --reference feat/strict-fix-multus
sudo microk8s enable hostpath-storage
sudo microk8s enable multus
```

### Setup local Docker registry

Install Docker

```shell
sudo snap install docker
```

Create a local registry

```shell
docker run -d -p 5000:5000 --name registry registry:2
```

### Install test pre-requisites

```shell
sudo snap install rockcraft --classic
```

### Build and Deploy Ella

Build the image and push it to the local registry

```shell
rockcraft pack
sudo rockcraft.skopeo --insecure-policy copy oci-archive:ella-core_v0.0.10_amd64.rock docker-daemon:ella-core:latest
docker tag ella-core:latest localhost:5000/ella-core:latest
docker push localhost:5000/ella-core:latest
```

Run End-to-End tests

```shell
pip install tox
tox -e integration
```

## How-to Guides

### Build the Frontend

Install pre-requisites:

```shell
sudo snap install node --channel=20/stable --classic
```

```shell
npm install --prefix ui
npm run build --prefix ui
```

### Build the Backend

Install pre-requisites:

```shell
sudo apt install clang llvm gcc-multilib libbpf-dev
sudo snap install go --channel=1.24/stable --classic
```

Generate the eBPF Go bindings:

```shell
go generate ./...
```

Build the backend:

```shell
go build cmd/core/main.go
```

### Build documentation

```shell
mkdocs build
```

### Build the Container image

```shell
sudo snap install rockcraft --classic --edge
rockcraft pack -v
sudo rockcraft.skopeo --insecure-policy copy oci-archive:ella-core_v0.0.10_amd64.rock docker-daemon:ella-core:latest
docker run ella-core:latest
```

### View Test Coverage

```shell
go test ./... -coverprofile coverage.out
go tool cover -func coverage.out
```

## References

### Embedded Database

Ella uses an embedded [SQLite](https://www.sqlite.org/) database to store its data. Type mappings between Go and SQLite are managed 
using [sqlair](https://github.com/canonical/sqlair).
