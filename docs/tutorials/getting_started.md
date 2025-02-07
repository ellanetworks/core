---
description: A hands-on introduction to Ella Core for new users.
---

# Getting Started

In this tutorial, we will deploy, initialize, and configure Ella Core. We will use [Multipass](https://canonical.com/multipass/docs) to create a virtual machine with multiple network interfaces, install Ella Core inside the virtual machine, access the UI, initialize Ella Core, and configure it.

You can expect to spend about 10 minutes completing this tutorial.

## Pre-requisites

To complete this tutorial, you will need a Ubuntu 24.04 machine with the following specifications:

- 8GB of RAM
- 4 CPU cores
- 30GB of disk space

## 1. Create a Virtual Machine

From the Ubuntu machine, install LXD and Multipass:

```shell
sudo snap install lxd
sudo snap install multipass
```

Initialize LXD:

```shell
sudo lxd init --auto
```

Create three networks:

```shell
lxc network create n2 ipv4.address=22.22.22.1/24
lxc network create n3 ipv4.address=33.33.33.1/24
lxc network create n6 ipv4.address=66.66.66.1/24
```

Use Multipass to create a bare Ubuntu 24.04 instance with two additional network interfaces:

```shell
multipass launch noble --name=ella-core --network n2 --network n3 --network n6
```

Validate that the instance has been created with the two additional network interfaces:

```shell
multipass list
```

You should see the following output:
```shell
Name                    State             IPv4             Image
ella-core               Running           10.194.229.141   Ubuntu 24.04 LTS
                                          22.22.22.42
                                          33.33.33.200
                                          66.66.66.218
```

You should see four IP addresses. Take note of the first one. In this example, the IP address is `10.194.229.141`. You will use this IP address later to access the Ella Core UI.

## 2. Install Ella Core

### 2.1 Install and start the Ella Core snap

Connect to the instance:

```shell
multipass shell ella-core
```

Validate that the instance has the two additional network interfaces:

Inside of the `ella-core` Multipass instance, install the Ella Core snap:

```shell
sudo snap install ella-core
```

Connect the snap to the required interfaces:

```bash
sudo snap connect ella-core:network-control
sudo snap connect ella-core:process-control
sudo snap connect ella-core:sys-fs-bpf-upf-pipeline
sudo snap connect ella-core:system-observe
```

Start Ella Core:

```shell
sudo snap start ella-core.cored
```

Validate that Ella Core is running:

```shell
sudo snap logs ella-core.cored
```

You should see that Ella Core has started:

```shell
2025-02-01T10:22:54-05:00 systemd[1]: Started snap.ella-core.cored.service - Service for snap application ella-core.cored.
2025-02-01T10:22:54-05:00 ella-core.cored[2512]: + /snap/ella-core/x1/bin/core -config /var/snap/ella-core/common/core.yaml
2025-02-01T10:22:55-05:00 ella-core.cored[2533]: 2025-02-01T10:22:55.333-0500	INFO	logger/logger.go:87	set log level: info	{"component": "Ella"}
2025-02-01T10:22:55-05:00 ella-core.cored[2533]: 2025-02-01T10:22:55.481-0500	INFO	db/operator.go:108	Initialized operator configuration	{"component": "DB"}
2025-02-01T10:22:55-05:00 ella-core.cored[2533]: 2025-02-01T10:22:55.482-0500	INFO	db/db.go:73	Database Initialized	{"component": "DB"}
2025-02-01T10:22:55-05:00 ella-core.cored[2533]: 2025-02-01T10:22:55.482-0500	INFO	nms/nms.go:38	API server started on https://localhost:5002	{"component": "NMS"}
2025-02-01T10:22:55-05:00 ella-core.cored[2533]: 2025-02-01T10:22:55.517-0500	INFO	service/service.go:68	NGAP server started on 10.194.229.141:38412	{"component": "AMF"}
```

Exit the Multipass instance:

```shell
exit
```

### 2.3 Access the UI

Navigate to `https://<your instance IP>:5002` to access Ella Core's UI. Use the IP address you noted earlier.

You should see the Initialization page.

![Initialize Ella Core](../images/initialize.png){ align=center }

!!! note
    Your browser may display a warning about the security of the connection. This is because the certificate used by Ella Core is self-signed. You can safely ignore this warning.

### 2.4 Initialize Ella Core

In the Initialization page, create the first user with the following credentials:

- Email: `admin@ellanetworks.com`
- Password: `admin`

After creating the user, you will be redirected to the login page. Use the credentials you just created to log in.

You will be redirected to the dashboard.

Ella Core is now initialized and ready to be used.

### 2.5 Create a Profile and a Subscriber

Here, we will navigate through the Ella Core UI to create a profile and a subscriber.

#### 2.5.1 Create a Profile

Navigate to the `Profiles` page and click on the `Create` button.

Create a profile with the name `default`. You can keep the default values for the other parameters:

- Name: `default`
- IP Pool: `10.45.0.0/16`
- DNS: `8.8.8.8`
- MTU: `1500`
- Bitrate Uplink: `200 Mbps`
- Bitrate Downlink: `100 Mbps`

#### 2.5.2 Create a subscriber

Navigate to the `Subscribers` page and click on the `Create` button.

Create a subscriber with the following parameters:

- IMSI: Click on "Generate" to automatically generate the MSIN.
- Key: Click on "Generate" to automatically generate a key.
- Sequence Number: Keep the default value.
- Profile: `default`

!!! success

    You have successfully deployed, initialized, and configured Ella Core. You can now use Ella Core to manage your private 5G network.

## 3. Destroy the Tutorial Environment

When you are done with the tutorial, you can destroy the Multipass instance:

```shell
multipass delete ella-core
```

You can also delete the networks created with LXD:

```shell
lxc network delete n2
lxc network delete n3
lxc network delete n6
```
