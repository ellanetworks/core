---
description: Deploy a complete 5G network with simulated radio and user equipment.
---

# Getting Started (Simulated 5G Network)

In this tutorial, we will deploy a complete end-to-end 5G network using Ella Core and UERANSIM (a 5G radio and user equipment simulator). By the end, a simulated subscriber will be connected to your network and able to reach the internet.

No 5G radio or specialized hardware is required.

You can expect to spend about 10 minutes completing this tutorial.

![Tutorial](../images/getting_started_sim.svg){ align=center }

## Pre-requisites

To complete this tutorial, you will need a Linux machine with [Docker](https://www.docker.com/) installed.

## 1. Install Ella Core and UERANSIM

Create a new directory for this tutorial and navigate into it:

```shell
mkdir ella
cd ella
```

Copy the following file into this directory:

```yaml title="docker-compose.yaml"
services:
  ella-core:
    image: ghcr.io/ellanetworks/ella-core:v1.10.0
    restart: unless-stopped
    entrypoint: /bin/core --config /core.yaml
    privileged: true
    ports:
      - "5002:5002"
    networks:
      default:
        driver_opts:
              com.docker.network.endpoint.ifname: eth0
      n3:
        driver_opts:
              com.docker.network.endpoint.ifname: n3
        ipv4_address: 10.3.0.2
  ueransim:
    image: ghcr.io/ellanetworks/ueransim:3.2.7
    restart: unless-stopped
    privileged: true
    networks:
      n3:
        driver_opts:
              com.docker.network.endpoint.ifname: n3
        ipv4_address: 10.3.0.3

networks:
  n3:
    internal: true
    ipam:
      config:
        - subnet: 10.3.0.0/24
```

Start the Ella Core and UERANSIM containers:

```shell
docker compose up -d
```

You should see the following output:

```shell
[+] Running 4/4
 ✔ Network ella_default        Created
 ✔ Network ella_n3             Created
 ✔ Container ella-ella-core-1  Started
 ✔ Container ella-ueransim-1   Started
```

## 2. Initialize Ella Core

Open your browser and navigate to `http://127.0.0.1:5002/` to access Ella Core's UI.

You should see the Initialization page.

![Initialize Ella Core](../images/initialize.png){ align=center }

!!! note
    Your browser may display a warning about the security of the connection. You can safely ignore this warning.

Create the first user with the following credentials:

- Email: `admin@ellanetworks.com`
- Password: `admin`

Ella Core is now initialized. You will be redirected to the dashboard.

![Dashboard](../images/dashboard.png){ align=center }

## 3. Create a Subscriber

Navigate to the `Subscribers` page and click on the `Create` button.

Create a subscriber with the following parameters:

- IMSI: `001019756139935`
- Key: `0eefb0893e6f1c2855a3a244c6db1277`
- Sequence Number: Keep the default value.
- OPC: Select "Provide custom OPC" and set the value to `98da19bbc55e2a5b53857d10557b1d26`.
- Profile: Keep the default value.

## 4. Connect the 5G Radio Simulator

Go back to your terminal, in the same directory where you created the `docker-compose.yaml` file.

Start the 5G radio simulator:

```shell
docker compose exec -ti ueransim bin/nr-gnb --config /gnb.yaml
```

You should see the following output:

```shell
UERANSIM v3.2.7
[2025-10-24 17:46:43.402] [sctp] [info] Trying to establish SCTP connection... (10.3.0.2:38412)
[2025-10-24 17:46:43.404] [sctp] [info] SCTP connection established (10.3.0.2:38412)
[2025-10-24 17:46:43.404] [sctp] [debug] SCTP association setup ascId[281]
[2025-10-24 17:46:43.404] [ngap] [debug] Sending NG Setup Request
[2025-10-24 17:46:43.405] [ngap] [debug] NG Setup Response received
[2025-10-24 17:46:43.405] [ngap] [info] NG Setup procedure is successful
```

Leave the radio running, don't close the terminal.

In your browser, navigate to the Ella Core UI and click on the `Radios` tab. You should see a radio connected with the name `UERANSIM-gnb-1-1-1`.

![Connected Radio](../images/connected_radio.png){ align=center }

## 5. Connect the User Equipment Simulator

Open a new terminal window in the same directory where you created the `docker-compose.yaml` file.

Start the UE simulator:

```shell
docker compose exec -ti ueransim bin/nr-ue --config /ue.yaml
```

You should see the following output:

```shell
UERANSIM v3.2.7
[2025-10-24 17:51:25.972] [nas] [info] UE switches to state [MM-DEREGISTERED/PLMN-SEARCH]
[2025-10-24 17:51:25.973] [rrc] [debug] New signal detected for cell[1], total [1] cells in coverage
[2025-10-24 17:51:25.973] [nas] [info] Selected plmn[001/01]
[2025-10-24 17:51:25.973] [rrc] [info] Selected cell plmn[001/01] tac[1] category[SUITABLE]
[2025-10-24 17:51:25.973] [nas] [info] UE switches to state [MM-DEREGISTERED/PS]
[2025-10-24 17:51:25.973] [nas] [info] UE switches to state [MM-DEREGISTERED/NORMAL-SERVICE]
[2025-10-24 17:51:25.973] [nas] [debug] Initial registration required due to [MM-DEREG-NORMAL-SERVICE]
[2025-10-24 17:51:25.973] [nas] [debug] UAC access attempt is allowed for identity[0], category[MO_sig]
[2025-10-24 17:51:25.973] [nas] [debug] Sending Initial Registration
[2025-10-24 17:51:25.973] [nas] [info] UE switches to state [MM-REGISTER-INITIATED]
[2025-10-24 17:51:25.973] [rrc] [debug] Sending RRC Setup Request
[2025-10-24 17:51:25.973] [rrc] [info] RRC connection established
[2025-10-24 17:51:25.973] [rrc] [info] UE switches to state [RRC-CONNECTED]
[2025-10-24 17:51:25.973] [nas] [info] UE switches to state [CM-CONNECTED]
[2025-10-24 17:51:25.975] [nas] [debug] Authentication Request received
[2025-10-24 17:51:25.975] [nas] [debug] Received SQN [000000000022]
[2025-10-24 17:51:25.975] [nas] [debug] SQN-MS [000000000000]
[2025-10-24 17:51:25.976] [nas] [debug] Security Mode Command received
[2025-10-24 17:51:25.976] [nas] [debug] Selected integrity[1] ciphering[0]
[2025-10-24 17:51:25.978] [nas] [debug] Registration accept received
[2025-10-24 17:51:25.978] [nas] [info] UE switches to state [MM-REGISTERED/NORMAL-SERVICE]
[2025-10-24 17:51:25.978] [nas] [debug] Sending Registration Complete
[2025-10-24 17:51:25.978] [nas] [info] Initial Registration is successful
[2025-10-24 17:51:25.978] [nas] [debug] Sending PDU Session Establishment Request
[2025-10-24 17:51:25.978] [nas] [debug] UAC access attempt is allowed for identity[0], category[MO_sig]
[2025-10-24 17:51:26.187] [nas] [debug] PDU Session Establishment Accept received
[2025-10-24 17:51:26.187] [nas] [info] PDU Session establishment is successful PSI[1]
[2025-10-24 17:51:26.211] [app] [info] Connection setup for PDU session[1] is successful, TUN interface[uesimtun0, 10.45.0.1] is up.
```

The User Equipment has successfully connected to the network and has been assigned the IP address `10.45.0.1`.

Leave the UE running, don't close the terminal.

## 6. Validate the Connection

In your browser, navigate to the Ella Core UI and click on the `Subscribers` tab. You should see that the subscriber you created has been assigned an IP address matching the one from the UE output.

![Connected Subscriber](../images/connected_subscriber.png){ align=center }

Open a new terminal window in the same directory where you created the `docker-compose.yaml` file.

Ping Google's DNS server from the subscriber's interface:

```shell
docker compose exec -ti ueransim ping -I uesimtun0 8.8.8.8 -c4
```

You should see a successful ping:

```shell
PING 8.8.8.8 (8.8.8.8) from 10.45.0.1 uesimtun0: 56(84) bytes of data.
64 bytes from 8.8.8.8: icmp_seq=1 ttl=116 time=39.0 ms
64 bytes from 8.8.8.8: icmp_seq=2 ttl=116 time=37.9 ms
64 bytes from 8.8.8.8: icmp_seq=3 ttl=116 time=37.4 ms
64 bytes from 8.8.8.8: icmp_seq=4 ttl=116 time=18.9 ms

--- 8.8.8.8 ping statistics ---
4 packets transmitted, 4 received, 0% packet loss, time 3003ms
rtt min/avg/max/mdev = 18.865/33.300/39.038/8.355 ms
```

!!! success

    Congratulations! You have deployed a complete end-to-end 5G network. A simulated subscriber is connected and can reach the internet through Ella Core.

## 7. Clean Up (Optional)

When you are done with the tutorial, you can remove the containers and the networks:

```shell
docker compose down
```
