---
description: A hands-on introduction to Ella Core for new users.
---

# Getting Started

In this tutorial, we will deploy, initialize, and configure Ella Core. We will use [Docker](https://www.docker.com/) to run a container with Ella Core, and we will access the Ella Core UI through a web browser.

You can expect to spend about 5 minutes completing this tutorial.

## Pre-requisites

To complete this tutorial, you will need a Linux machine with Docker.

## 1. Install Ella Core

Create two networks:

```shell
docker network create --driver bridge n3 --subnet 10.3.0.0/24
```

Start the Ella Core container with the additional network interfaces:

```shell
docker create \
  --name ella-core \
  --privileged \
  -p 5002:5002 \
  -v /sys/fs/bpf:/sys/fs/bpf:rw \
  ella-core:latest exec /bin/core --config /core.yaml
docker network connect --ip 10.3.0.2 n3 ella-core
docker start ella-core
```

## 2. Access the UI

Open your browser and navigate to `http://127.0.0.1:5002` to access Ella Core's UI.

You should see the Initialization page.

![Initialize Ella Core](../images/initialize.png){ align=center }

!!! note
    Your browser may display a warning about the connection's security. You can safely ignore this warning.

## 3 Initialize Ella Core

On the Initialization page, create the first user with the following credentials:

- Email: `admin@ellanetworks.com`
- Password: `admin`

After creating the user, Ella Core will redirect you to the dashboard.

![Dashboard](../images/dashboard.png){ align=center }

!!! success

    You have successfully deployed and initialized Ella Core. You can now use Ella Core to manage your private 5G network.

## 4. Destroy the Tutorial Environment (Optional)

When you are done with the tutorial, you can remove the Ella Core container and the networks we created.

```shell
docker stop ella-core
docker rm ella-core
docker network rm n3
```
