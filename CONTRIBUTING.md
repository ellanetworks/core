# Contributing

## Getting Started

### Set up MicroK8s

```shell
sudo snap install microk8s --channel=1.31-strict/stable
```

```shell
Add the community repository MicroK8s addon:
sudo microk8s addons repo add community https://github.com/canonical/microk8s-community-addons --reference feat/strict-fix-multus
```

Enable the following MicroK8s addons. We must give MetalLB an address range that has at least 3 IP addresses for Charmed Aether SD-Core.

```shell
sudo microk8s enable hostpath-storage
sudo microk8s enable multus
sudo microk8s enable metallb:10.0.0.2-10.0.0.4
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
make oci-build
```

Deploy Ella and its dependencies

```shell
make deploy
```

## How-to Guides

### Build

```
go install github.com/swaggo/swag/cmd/swag@v1.8.12
export PATH=$(go env GOPATH)/bin:$PATH
go generate -v ./internal/upf/...
go build cmd/ella/main.go
```

### Generate database code


Generate the sqlc code:

```shell
sqlc generate
```

### Build Frontend

```
cd ui
npm install
npm run build
cp -r out/* ../internal/nms/ui/frontend_files/
```

### Build the Container image

```bash
sudo snap install rockcraft --classic --edge
rockcraft pack -v
sudo rockcraft.skopeo --insecure-policy copy oci-archive:ella_0.0.4_amd64.rock docker-daemon:ella:0.0.4
docker run ella:0.0.4
```

## References

### SQLC

- [SQLC](https://docs.sqlc.dev/en/latest/)
