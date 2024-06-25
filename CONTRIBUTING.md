# Contributing

## Build

```
go install github.com/swaggo/swag/cmd/swag@v1.8.12
export PATH=$(go env GOPATH)/bin:$PATH
go generate -v ./internal/upf/...
go build cmd/ella/main.go
```