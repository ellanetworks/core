<!--
SPDX-License-Identifier: Apache-2.0
Copyright 2021-present Open Networking Foundation
-->

# Protobuf for sd core config grpc server & client

The config.proto file contains the messages and methods to be used by the
grpc server and client for exchange of config info.
To add updates, just change the file and run in webconsole folder the following
command : 
    make -f Makefile_docker docker-build

The Dockerfile contains the commands to generate the golang files from this config.proto. 
The commands are as follows : 

    protoc -I ./ --go_out=. config.proto # This generates the messages
    protoc -I ./ --go-grpc_out=. config.proto # This generates the services

To run the above commands, we install the protoc compiler and the protobuf go
based plugin. The commands for installing are as follows : 

    curl -OL https://github.com/protocolbuffers/protobuf/releases/download/v3.14.0/protoc-3.14.0-linux-x86_64.zip
    unzip -o protoc-3.14.0-linux-x86_64.zip -d ./proto 
    chmod 755 -R ./proto/bin
    sudo cp ./proto/bin/protoc /usr/bin/
    sudo cp -R ./proto/include/* /usr/include/
    go get -u google.golang.org/protobuf/cmd/protoc-gen-go
    go install google.golang.org/protobuf/cmd/protoc-gen-go
    go get -u google.golang.org/grpc/cmd/protoc-gen-go-grpc
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc

The protoc compiler generates two files in the sdcoreConfig directory which
contain the messages and methods to be used by the server and client :
    config.pb.go
    config_grpc.pb.go

The gClient.go file under client folder is not a generated file. It exposes the
client APIs to be used by any application which will behave as the grpc client. 
