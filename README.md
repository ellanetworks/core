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
