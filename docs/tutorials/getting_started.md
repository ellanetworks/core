---
description: A hands-on introduction to Ella Core for new users.
---

# Getting Started

In this tutorial, we will deploy, initialize, and configure Ella Core. We will use [Multipass](https://canonical.com/multipass/docs) to create a virtual machine with multiple network interfaces, install Ella Core inside the virtual machine, access the UI, initialize Ella Core, and configure it.

You can expect to spend about 10 minutes completing this tutorial.

## Pre-requisites

To complete this tutorial, you will need a Ubuntu 24.04 machine with the following specifications:

- **Memory**: 8GB
- **CPU**: 4 cores
- **Disk**: 30GB

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
multipass launch noble --name=ella-core --disk=10G --cpus 2 --network n2 --network n3 --network n6
```

## 2. Install Ella Core

### 2.1 Install and start the Ella Core snap

Connect to the instance:

```shell
multipass shell ella-core
```

Inside the `ella-core` Multipass instance, install the Ella Core snap and connect it to the required interfaces

```shell
sudo snap install ella-core
sudo snap connect ella-core:network-control
sudo snap connect ella-core:process-control
sudo snap connect ella-core:sys-fs-bpf-upf-pipeline
sudo snap connect ella-core:system-observe
sudo snap connect ella-core:firewall-control
```

Start Ella Core:

```shell
sudo snap start ella-core.cored
```

Exit the Multipass instance:

```shell
exit
```

### 2.2 Access the UI

Get the IP address of the Multipass instance:

```shell
multipass info ella-core
```

Note the first IP address.

Navigate to `https://<your instance IP>:5002` to access Ella Core's UI.

You should see the Initialization page.

![Initialize Ella Core](../images/initialize.png){ align=center }

!!! note
    Your browser may display a warning about the connection's security because Ella Core uses a self-signed certificate. You can safely ignore this warning.

### 2.3 Initialize Ella Core

On the Initialization page, create the first user with the following credentials:

- Email: `admin@ellanetworks.com`
- Password: `admin`

After creating the user, Ella Core will redirect you to the Login page. Use the credentials you just created to log in.

Ella Core should redirect you to the dashboard.

![Dashboard](../images/dashboard.png){ align=center }


!!! success

    You have successfully deployed and initialized Ella Core. You can now use Ella Core to manage your private 5G network.

## 3. Destroy the Tutorial Environment (Optional)

When you are done with the tutorial, you can destroy the Multipass instance:

```shell
multipass delete ella-core --purge
```

You can also delete the networks created with LXD:

```shell
lxc network delete n2
lxc network delete n3
lxc network delete n6
```
