#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

SEPP_FQDN="sepp1.5gc.mnc001.mcc001.3gppnetwork.org"
ELLA_FQDN="sepp1.5gc.mnc070.mcc999.3gppnetwork.org"
SEPP_PORT="${SEPP_HOST_PORT:-7777}"
SEPP_ADDR="${SEPP_HOST_BIND:-127.0.0.1}"
AUSF_APIROOT="http://ausf.5gc.mnc001.mcc001.3gppnetwork.org"
NRF_APIROOT="http://nrf.5gc.mnc001.mcc001.3gppnetwork.org"

n32() {
  curl -sS --http2 -m 15 \
    --cert certs/ella-sepp.crt --key certs/ella-sepp.key --cacert certs/open5gs-ca.crt \
    --resolve "$SEPP_FQDN:$SEPP_PORT:$SEPP_ADDR" -w '\nHTTP %{http_code}\n' "$@"
}

wait_registered() {
  local svc ok
  for svc in udr udm ausf sepp; do
    ok=0
    for _ in $(seq 1 30); do
      if docker compose logs "$svc" 2>/dev/null | grep -q 'NF registered \[Heartbeat'; then
        ok=1
        break
      fi
      sleep 1
    done
    if [ "$ok" = 1 ]; then echo "$svc registered with NRF"; else echo "$svc NOT registered with NRF" >&2; exit 1; fi
  done
}

case "${1:-}" in
  up)
    [ -f certs/ella-ca.crt ] || ./gen-certs.sh
    docker compose up -d
    wait_registered
    ;;
  down)
    docker compose down -v --remove-orphans
    ;;
  logs)
    docker compose logs -f "${@:2}"
    ;;
  test)
    echo "== N32-c exchange-capability"
    n32 -H 'content-type: application/json' \
      "https://$SEPP_FQDN:$SEPP_PORT/n32c-handshake/v1/exchange-capability" \
      -d "{\"sender\":\"$ELLA_FQDN\",\"supportedSecCapabilityList\":[\"TLS\"],\"3GppSbiTargetApiRootSupported\":true,\"plmnIdList\":[{\"mcc\":\"999\",\"mnc\":\"70\"}],\"targetPlmnId\":{\"mcc\":\"001\",\"mnc\":\"01\"},\"supportedFeatures\":\"1\"}"
    echo "== N32-f NRF discovery (AUSF)"
    n32 -G -H "3gpp-Sbi-Target-apiRoot: $NRF_APIROOT" \
      "https://$SEPP_FQDN:$SEPP_PORT/nnrf-disc/v1/nf-instances" \
      --data-urlencode 'target-nf-type=AUSF' \
      --data-urlencode 'requester-nf-type=AMF' \
      --data-urlencode 'target-plmn-list=[{"mcc":"001","mnc":"01"}]' \
      --data-urlencode 'requester-plmn-list=[{"mcc":"999","mnc":"70"}]'
    echo "== N32-f AUSF ue-authentications"
    n32 -H "3gpp-Sbi-Target-apiRoot: $AUSF_APIROOT" -H 'content-type: application/json' \
      "https://$SEPP_FQDN:$SEPP_PORT/nausf-auth/v1/ue-authentications" \
      -d '{"supiOrSuci":"imsi-001010000000001","servingNetworkName":"5G:mnc070.mcc999.3gppnetwork.org"}'
    ;;
  *)
    echo "usage: $0 {up|down|test|logs [service]}" >&2
    exit 1
    ;;
esac
