# Common operations

All examples assume `ELLA_CORE_URL` and `ELLA_CORE_TOKEN` are set. `curl -k` is used because Ella Core typically serves HTTPS with a self-signed certificate. If you need an endpoint not listed here, fetch the OpenAPI spec.

## Status (unauthenticated)

```bash
curl -sk "$ELLA_CORE_URL/api/v1/status" | jq
```

## Identify the calling user

```bash
curl -sk -H "Authorization: Bearer $ELLA_CORE_TOKEN" \
  "$ELLA_CORE_URL/api/v1/users/me" | jq
```

## List subscribers (paginated)

```bash
curl -sk -H "Authorization: Bearer $ELLA_CORE_TOKEN" \
  "$ELLA_CORE_URL/api/v1/subscribers?page=1&per_page=100" | jq
```

## Get a subscriber by IMSI

```bash
curl -sk -H "Authorization: Bearer $ELLA_CORE_TOKEN" \
  "$ELLA_CORE_URL/api/v1/subscribers/999016992280505" | jq
```

## Create a subscriber

Note the **mixed casing** — `sequenceNumber` is camelCase, the rest are snake_case.

```bash
curl -sk -X POST -H "Authorization: Bearer $ELLA_CORE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "imsi": "001010000000001",
    "key": "00112233445566778899aabbccddeeff",
    "sequenceNumber": "000000000020",
    "profile_name": "default"
  }' \
  "$ELLA_CORE_URL/api/v1/subscribers" | jq
```

## Create a policy

```bash
curl -sk -X POST -H "Authorization: Bearer $ELLA_CORE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-policy",
    "profile_name": "default",
    "slice_name": "default",
    "data_network_name": "internet",
    "session_ambr_uplink": "50 Mbps",
    "session_ambr_downlink": "100 Mbps",
    "var5qi": 9,
    "arp": 1
  }' \
  "$ELLA_CORE_URL/api/v1/policies" | jq
```

## List data networks

```bash
curl -sk -H "Authorization: Bearer $ELLA_CORE_TOKEN" \
  "$ELLA_CORE_URL/api/v1/networking/data-networks" | jq
```

## Subscriber data usage (last 7 days, by subscriber)

`group_by` is **required**.

```bash
curl -sk -H "Authorization: Bearer $ELLA_CORE_TOKEN" \
  "$ELLA_CORE_URL/api/v1/subscriber-usage?group_by=subscriber" | jq

# By day with explicit range
curl -sk -H "Authorization: Bearer $ELLA_CORE_TOKEN" \
  "$ELLA_CORE_URL/api/v1/subscriber-usage?group_by=day&start=$(date -u -d '7 days ago' +%F)&end=$(date -u +%F)" | jq
```

## Iterate all pages

```bash
page=1
while :; do
  resp=$(curl -sk -H "Authorization: Bearer $ELLA_CORE_TOKEN" \
    "$ELLA_CORE_URL/api/v1/subscribers?page=$page&per_page=100")
  echo "$resp" | jq '.result.items[]'
  total=$(echo "$resp" | jq '.result.total_count')
  (( page * 100 >= total )) && break
  page=$((page + 1))
done
```

## Discover an unknown endpoint

```bash
curl -sk "$ELLA_CORE_URL/api/v1/openapi.yaml" \
  | yq '.paths | keys' 2>/dev/null \
  || curl -sk "$ELLA_CORE_URL/api/v1/openapi.yaml" | grep -E '^  /api'
```
