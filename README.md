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
3. **Performance**: We are commited to develop a high performance mobile network. We aim to minimize latency, maximize throughput, and minimize resource usage. 

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
tls:
  cert: "testdata/cert.pem"
  key: "testdata/key.pem"
db:
  url: "mongodb://localhost:27017"
  name: "test"
upf:
  interfaces: ["enp3s0"]
  n3-address: "127.0.0.1"
```
