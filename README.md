# Ella

ella is a secure, reliable, and easy to operate mobile network.


## Getting Started

Install the snap:

```bash
sudo snap install ella
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

## Reference

### Configuration

Example:

```yaml
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

### API

#### Inventory (radios)
- GET `/inventory/radios`: List all radios
- GET `/inventory/radios/{id}`: Get a radio from inventory
- DELETE `/inventory/radios/{id}`: Delete a radio from inventory
- POST `/inventory/radios`: Create a radio in inventory

#### Subscribers
- GET `/subscribers`: List all subscribers
- GET `/subscribers/{id}`: Get a subscriber
- DELETE `/subscribers/{id}`: Delete a subscriber
- POST `/subscribers`: Create a subscriber

#### Device Groups
- GET `/device-groups`: List all device groups
- GET `/device-groups/{id}`: Get a device group
- DELETE `/device-groups/{id}`: Delete a device group
- POST `/device-groups`: Create a device group

#### Network Slices
- GET `/network-slices`: List all network slices
- GET `/network-slices/{id}`: Get a network slice
- DELETE `/network-slices/{id}`: Delete a network slice
- POST `/network-slices`: Create a network slice
