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
3. **Performance**: We are commited to develop a high performance mobile network. We aim to minimize latency, maximize throughput, and minimize resource usage. We use a data-driven approach to measure and improve performance.

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

#### Configuration

Example:

```yaml
port: 5000
tls:
  cert: "testdata/cert.pem"
  key: "testdata/key.pem"
db:
  mongo:
    url: "mongodb://localhost:27017"
    name: "test"
  sql:
    path: "testdata/sqlite.db"
upf:
  interfaces: ["eth0", "eth1"]
  n3-address: "127.0.0.1"
```

#### API

| Endpoint                        | HTTP Method | Description                   | Parameters |
| ------------------------------- | ----------- | ----------------------------- | ---------- |
| `/api/v1/inventory/radios`      | GET         | List all radios               |            |
| `/api/v1/inventory/radios/{id}` | GET         | Get a radio from inventory    |            |
| `/api/v1/inventory/radios/{id}` | DELETE      | Delete a radio from inventory |            |
| `/api/v1/inventory/radios`      | POST        | Create a radio in inventory   |            |
| `/api/v1/subscribers`           | GET         | List all subscribers          |            |
| `/api/v1/subscribers/{id}`      | GET         | Get a subscriber              |            |
| `/api/v1/subscribers/{id}`      | DELETE      | Delete a subscriber           |            |
| `/api/v1/subscribers`           | POST        | Create a subscriber           |            |
| `/api/v1/device-groups`         | GET         | List all device groups        |            |
| `/api/v1/device-groups/{id}`    | GET         | Get a device group            |            |
| `/api/v1/device-groups/{id}`    | DELETE      | Delete a device group         |            |
| `/api/v1/device-groups`         | POST        | Create a device group         |            |
| `/api/v1/network-slices`        | GET         | List all network slices       |            |
| `/api/v1/network-slices/{id}`   | GET         | Get a network slice           |            |
| `/api/v1/network-slices/{id}`   | DELETE      | Delete a network slice        |            |
| `/api/v1/network-slices`        | POST        | Create a network slice        |            |
| `/api/v1/metrics`               | GET         | Get metrics                   |            |
| `/api/v1/status`                | GET         | Get status                    |            |
