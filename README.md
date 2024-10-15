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

#### Inventory (gNBs)
- GET `/inventory/gnbs`: List all gNBs
- GET `/inventory/gnbs/{id}`: Get a gNB
- DELETE `/inventory/gnbs/{id}`: Delete a gNB
- POST `/inventory/gnbs`: Create a gNB

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
- GET `/device-groups/{device_group_id}/subscribers`: List all subscribers in a device group
- POST `/device-groups/{device_group_id}/subscribers`: Add a subscriber to a device group
- DELETE `/device-groups/{device_group_id}/subscribers/{subscriber_id}`: Remove a subscriber from a device group

#### Network Slices
- GET `/network-slices`: List all network slices
- GET `/network-slices/{id}`: Get a network slice
- DELETE `/network-slices/{id}`: Delete a network slice
- POST `/network-slices`: Create a network slice
