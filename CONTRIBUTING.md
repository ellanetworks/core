# Contributing

## Build

```
go install github.com/swaggo/swag/cmd/swag@v1.8.12
export PATH=$(go env GOPATH)/bin:$PATH
go generate -v ./internal/upf/...
go build cmd/ella/main.go
```

### Frontend

```
cd ui
npm install
npm run build
cp -r out/* ../internal/webui/ui/frontend_files/
```

### Container image

```bash
sudo snap install rockcraft --classic --edge
rockcraft pack -v
sudo rockcraft.skopeo --insecure-policy copy oci-archive:ella_0.0.1_amd64.rock docker-daemon:ella:0.0.1
docker run ella:0.0.1
```
