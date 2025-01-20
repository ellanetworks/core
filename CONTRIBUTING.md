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
sudo rockcraft.skopeo --insecure-policy copy oci-archive:$(ROCK_FILE) docker-daemon:$(OCI_IMAGE_NAME)
docker tag ella-core:latest localhost:5000/ella-core:latest
docker push localhost:5000/ella-core:latest
```

Run End-to-End tests

```shell
kubectl create ns dev2
pip install tox
tox -e integration
```

## How-to Guides

### Build Backend

```shell
go build cmd/core/main.go
```

### Build Frontend

```shell
cd ui
npm install
npm run build --prefix ui
```

### Build documentation

```shell
mkdocs build
```

### Build the Container image

```shell
sudo snap install rockcraft --classic --edge
rockcraft pack -v
sudo rockcraft.skopeo --insecure-policy copy oci-archive:ella-core_0.0.4_amd64.rock docker-daemon:ella-core:latest
docker run ella-core:latest
```

## References

### Embedded Database

Ella uses an embedded [SQLite](https://www.sqlite.org/) database to store its data. Type mappings between Go and SQLite are managed 
using [sqlair](https://github.com/canonical/sqlair).
