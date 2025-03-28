---
description: Running an end-to-end 5G network with Ella Core on Kubernetes
---

# Running an End-to-End 5G Network with Ella Core (Kubernetes)

In this tutorial, we will deploy, initialize, and configure Ella Core, an open-source 5G mobile core network. First, we will use install Kubernetes, and create deployments for Ella Core, a router, and a 5G radio simulator. Then, we will configure Ella Core, and use the simulator to validate that subscribers can communicate with the Internet using Ella Core.

You can expect to spend about 30 minutes completing this tutorial. Follow the steps in sequence to ensure a successful deployment.

![Tutorial](../images/tutorial.svg){ align=center }

## Pre-requisites

To complete this tutorial, you will need a Ubuntu machine with the following specifications:

- **Memory**: 16GB
- **CPU**: 6 cores
- **Disk**: 50GB

## 1. Install Kubernetes

From your Ubuntu machine, install Kubernetes:

```shell
sudo snap install microk8s --channel=1.32/stable --classic
```

Enable the required addons:

```shell
sudo microk8s addons repo add community https://github.com/canonical/microk8s-community-addons --reference feat/strict-fix-multus
sudo microk8s enable hostpath-storage
sudo microk8s enable multus
```

## 2. Install Ella Core

Create a Namespace for Ella Core:

```shell
microk8s.kubectl create namespace ella
```

Install Ella Core:

```shell
microk8s.kubectl apply -k https://raw.githubusercontent.com/ellanetworks/core/refs/heads/main/k8s/
```
