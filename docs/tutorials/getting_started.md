---
description: A hands-on introduction to Ella Core for new users.
---

# Getting Started

In this tutorial, we will deploy, initialize, and configure Ella Core.

## Pre-requisites

- A Ubuntu 24.04 machine
  - 8GB of RAM
  - 4 CPU cores
  - 30GB of disk space
  - 4 network interfaces

## Install Ella Core
Install the snap:

```bash
sudo snap install ella-core --channel=edge --devmode
```

Generate (or copy) a certificate and private key to the following location:
```bash
sudo openssl req -newkey rsa:2048 -nodes -keyout /var/snap/ella-core/common/key.pem -x509 -days 1 -out /var/snap/ella-core/common/cert.pem -subj "/CN=example.com"
```

Edit the configuration file at `/var/snap/ella-core/common/config.yaml` to configure the network interfaces:
```yaml
log-level: "info"
db:
  path: "core.db"
interfaces: 
  n3: 
    name: "eth0" # Change this to the name of the network interface connected to the radios
    address: "127.0.0.1"
  n6:
    name: "eth1" # Change this to the name of the network interface connected to the internet
  api:
    name: "lo"
    port: 5002
    tls:
      cert: "/var/snap/ella-core/common/cert.pem"
      key: "/var/snap/ella-core/common/key.pem"
```

Start the service:
```bash
sudo snap start ella-core.cored
```

## Initialize Ella Core

Navigate to `https://localhost:5002` to access Ella Core's UI.

You should be prompted to initialize Ella Core and to create the first user.

Create the first user with the following credentials:

- Email: `admin@ellanetworks.com`
- Password: `admin`

After creating the user, you will be redirected to the login page. Use the credentials you just created to log in.

You will be redirected to the dashboard.

## Create a Radio

Navigate to the `Radios` page and click on the `Create` button.

Create a radio with the following parameters:

- Name: `test`
- TAC: `001`

## Create a Profile

Navigate to the `Profiles` page and click on the `Create` button.

Create a profile with the following parameters:

- Name: `default`
- UE IP Pool: `172.250.0.0/16`
- DNS: `8.8.8.8`
- MTU: `1500`
- Bitrate Uplink: `200 Mbps`
- Bitrate Downlink: `100 Mbps`

## Create a subscriber

Navigate to the `Subscribers` page and click on the `Create` button.

Create a subscriber with the following parameters:

- IMSI: Click on "Generate" to automatically generate an IMSI.
- Key: Click on "Generate" to automatically generate a key.
- Sequence Number: Keep the default value.
- Profile: `default`
