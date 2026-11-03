#!/usr/bin/env bash
set -euo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)/certs"
DAYS="${DAYS:-825}"
mkdir -p "$DIR"
cd "$DIR"

ELLA_FQDN="sepp1.5gc.mnc070.mcc999.3gppnetwork.org"
O5GS_FQDN="sepp1.5gc.mnc001.mcc001.3gppnetwork.org"

make_ca() {
  local name="$1" cn="$2"
  openssl ecparam -name prime256v1 -genkey -noout -out "$name-ca.key"
  openssl req -x509 -new -key "$name-ca.key" -sha256 -days "$DAYS" \
    -subj "/O=$cn/CN=$cn Root CA" \
    -addext "basicConstraints=critical,CA:TRUE,pathlen:0" \
    -addext "keyUsage=critical,keyCertSign,cRLSign" \
    -addext "subjectKeyIdentifier=hash" \
    -out "$name-ca.crt"
}

make_leaf() {
  local name="$1" fqdn="$2" cn="$3"
  openssl ecparam -name prime256v1 -genkey -noout -out "$name-sepp.key.ec"
  openssl pkcs8 -topk8 -nocrypt -in "$name-sepp.key.ec" -out "$name-sepp.key"
  rm -f "$name-sepp.key.ec"
  openssl req -new -key "$name-sepp.key" -subj "/O=$cn/CN=$fqdn" -out "$name-sepp.csr"
  cat > "$name-sepp.ext" <<EXT
basicConstraints=critical,CA:FALSE
keyUsage=critical,digitalSignature
extendedKeyUsage=serverAuth,clientAuth
subjectAltName=DNS:$fqdn
subjectKeyIdentifier=hash
authorityKeyIdentifier=keyid
EXT
  openssl x509 -req -in "$name-sepp.csr" -CA "$name-ca.crt" -CAkey "$name-ca.key" \
    -CAcreateserial -days "$DAYS" -sha256 -extfile "$name-sepp.ext" -out "$name-sepp.crt"
  rm -f "$name-sepp.csr" "$name-sepp.ext" "$name-ca.srl"
}

make_ca ella "Ella"
make_leaf ella "$ELLA_FQDN" "Ella"
make_ca open5gs "Open5GS"
make_leaf open5gs "$O5GS_FQDN" "Open5GS"

chmod 644 ./*.crt ./*.key
openssl verify -CAfile ella-ca.crt ella-sepp.crt
openssl verify -CAfile open5gs-ca.crt open5gs-sepp.crt
