# Ella

Ella is a wireless private mobile network.

Typical mobile networks are complex, expensive, and difficult to operate. Forget microservices, external databases, complex configurations, and expensive hardware. Ella is a single binary that runs on a single machine.

Use Ella where you need 5G connectivity: in a factory, a warehouse, a farm, a stadium, a ship, or a remote location.

## Key features

* **5G compliant**: Ella is a 5G compliant core network. It can integrate with any 5G radio access network.
* **Performant data plane**: Ella uses eBPF to implement the data plane. It is fast, secure, and reliable.
* **Simple UI**: Ella has a web-based user interface for managing subscribers, radios, device groups, and network slices.
* **Complete HTTP API**: Ella has a complete HTTP API. You can automate everything you can do in the UI.
* **Encrypted communication**: Ella's HTTP API and UI are secured with TLS.

## Tenets

1. **Simplicity**: We are commited to develop the simplest possible mobile network out there. We thrive on having a very short "Getting Started" tutorial, a simple configuration file, a single binary, and a simple UI.
2. **Reliability**: We are commited to develop a reliable mobile network you can trust to work 24/7. We are commited to deliver high quality code, tests, and documentation. We are commited to expose useful metrics and logs to help users monitor their network.
3. **Security**: We are commited to minimizing the attack surface of the private network and to use secure encryption protocols to protect the data of our users.

## Documentation

### Getting Started

Install the snap:

```bash
sudo snap install ella --channel=edge --devmode
```

Generate (or copy) a certificate and private key to the following location:
```bash
sudo openssl req -newkey rsa:2048 -nodes -keyout /var/snap/ella/common/key.pem -x509 -days 1 -out /var/snap/ella/common/cert.pem -subj "/CN=example.com"
```

Start the service:
```bash
sudo snap start ella.ellad
```

Navigate to `https://localhost:5000` to access the Ella UI.

### Reference

#### API

| Endpoint                     | HTTP Method | Description             |
| ---------------------------- | ----------- | ----------------------- |
| `/api/v1/subscriber`         | GET         | List subscribers        |
| `/api/v1/subscriber`         | POST        | Create a new subscriber |
| `/api/v1/subscriber/{id}`    | GET         | Get a subscriber        |
| `/api/v1/subscriber/{id}`    | PUT         | Update a subscriber     |
| `/api/v1/subscriber/{id}`    | DELETE      | Delete a subscriber     |
| `/api/v1/inventory/gnb`      | GET         | List radios             |
| `/api/v1/inventory/gnb`      | POST        | Create a new radio      |
| `/api/v1/inventory/gnb/{id}` | GET         | Get a radio             |
| `/api/v1/inventory/gnb/{id}` | DELETE      | Delete a radio          |
| `/api/v1/network-slice`      | GET         | List network slices     |
| `/api/v1/network-slice`      | POST        | Create a new slice      |
| `/api/v1/network-slice/{id}` | GET         | Get a slice             |
| `/api/v1/network-slice/{id}` | PUT         | Update a slice          |
| `/api/v1/network-slice/{id}` | DELETE      | Delete a slice          |
| `/api/v1/device-group`       | GET         | List device groups      |
| `/api/v1/device-group`       | POST        | Create a new group      |
| `/api/v1/device-group/{id}`  | GET         | Get a group             |
| `/api/v1/device-group/{id}`  | PUT         | Update a group          |
| `/api/v1/device-group/{id}`  | DELETE      | Delete a group          |


#### Configuration

Example:

```yaml
db:
  url: "mongodb://localhost:27017"
  name: "test"
upf:
  interfaces: ["enp3s0"]
  n3-address: "127.0.0.1"
```

#### Connectivity

Ella uses the following ports:
- **5000**: HTTPs API and UI
- **38412**: N2 interface between Ella and 5G Radio (Control Plane)
- **2152**: N3 interface between Ella and 5G Radio (User Plane)

![alt text](connectivity.png)
