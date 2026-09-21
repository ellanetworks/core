#!/bin/busybox sh
set -eu

BB=/bin/busybox
IP=/bin/ip

: "${VRF_UP_NAME:=up-vrf}"
: "${VRF_TABLE_UP:=1001}"
: "${VRF_UP_IFACES:=n3 n6}"
: "${VRF_CP_NAME:=cp-vrf}"
: "${VRF_TABLE_CP:=1002}"
: "${VRF_CP_IFACES:=eth0}"
: "${VRF_UP_ROUTES:=}"
: "${VRF_MAIN_DEFAULT:=}"
: "${VRF_MAIN_DEFAULT_IFACE:=mgmt0}"
: "${VRF_MAIN_DEFAULT_ADDR:=10.200.0.1/24}"
: "${VRF_MAIN_DEFAULT_GW:=10.200.0.254}"
: "${CORE_BIN:=/bin/core}"
: "${CORE_CONFIG:=/core.yaml}"

log() { echo "[vrf-setup] $*" >&2; }

collect_connected() {
  for iface in "$@"; do
    if $IP link show "$iface" >/dev/null 2>&1; then
      for prefix in $($IP route show table main dev "$iface" proto kernel 2>/dev/null | $BB awk '$1 ~ /\// {print $1}'); do
        echo "4|$iface|$prefix"
      done
      for prefix in $($IP -6 route show table main dev "$iface" proto kernel 2>/dev/null | $BB awk '$1 ~ /\// && $1 !~ /^fe80:/ && $1 !~ /^ff00:/ {print $1}'); do
        echo "6|$iface|$prefix"
      done
    else
      log "interface $iface absent, skipping"
    fi
  done
}

migrate_connected() {
  table=$1
  entries=$2
  echo "$entries" | while IFS= read -r entry; do
    if [ -z "$entry" ]; then
      continue
    fi
    old_ifs=$IFS
    IFS='|'
    set -- $entry
    IFS=$old_ifs
    if [ "$1" = "6" ]; then
      $IP -6 route add "$3" dev "$2" table "$table" 2>/dev/null || true
    else
      $IP route add "$3" dev "$2" table "$table" 2>/dev/null || true
    fi
  done
}

$BB sysctl -w net.ipv4.ip_forward=1
$BB sysctl -w net.ipv6.conf.all.forwarding=1
$BB sysctl -w net.ipv6.conf.all.keep_addr_on_down=1
$BB sysctl -w net.ipv6.conf.default.keep_addr_on_down=1
$BB sysctl -w net.vrf.strict_mode=1 || true

for keep_iface in $VRF_UP_IFACES $VRF_CP_IFACES; do
  if $IP link show "$keep_iface" >/dev/null 2>&1; then
    $BB sysctl -w "net.ipv6.conf.$keep_iface.keep_addr_on_down=1"
  fi
done

DEFAULT_GWS=""
for iface in $VRF_CP_IFACES; do
  if $IP link show "$iface" >/dev/null 2>&1; then
    gw=$($IP route show default dev "$iface" 2>/dev/null | $BB awk '{print $3; exit}') || true
    if [ -n "${gw:-}" ]; then
      DEFAULT_GWS="$DEFAULT_GWS $iface:$gw"
    fi
    gw6=$($IP -6 route show default dev "$iface" 2>/dev/null | $BB awk '{print $3; exit}') || true
    if [ -n "${gw6:-}" ]; then
      DEFAULT_GWS="$DEFAULT_GWS $iface:$gw6"
    fi
  else
    log "control-plane interface $iface absent, skipping"
  fi
done

$IP link add "$VRF_UP_NAME" type vrf table "$VRF_TABLE_UP"
$IP link set "$VRF_UP_NAME" up
$IP link add "$VRF_CP_NAME" type vrf table "$VRF_TABLE_CP"
$IP link set "$VRF_CP_NAME" up

UP_CONNECTED=$(collect_connected $VRF_UP_IFACES)
for iface in $VRF_UP_IFACES; do
  if $IP link show "$iface" >/dev/null 2>&1; then
    $IP link set "$iface" master "$VRF_UP_NAME"
  fi
done
migrate_connected "$VRF_TABLE_UP" "$UP_CONNECTED"

CP_CONNECTED=$(collect_connected $VRF_CP_IFACES)
for iface in $VRF_CP_IFACES; do
  if $IP link show "$iface" >/dev/null 2>&1; then
    $IP link set "$iface" master "$VRF_CP_NAME"
  fi
done
migrate_connected "$VRF_TABLE_CP" "$CP_CONNECTED"

if [ -n "${VRF_UP_ROUTES:-}" ]; then
  echo "$VRF_UP_ROUTES" | $BB tr ';' '\n' | while IFS= read -r route; do
    if [ -n "$route" ]; then
      case "$route" in
        *:*)
          $IP -6 route add $route table "$VRF_TABLE_UP"
          ;;
        *)
          $IP route add $route table "$VRF_TABLE_UP"
          ;;
      esac
    fi
  done
fi

for entry in $DEFAULT_GWS; do
  iface=${entry%%:*}
  gw=${entry#*:}
  if [ -n "${iface:-}" ] && [ -n "${gw:-}" ]; then
    case "$gw" in
      *:*)
        $IP -6 route add default via "$gw" dev "$iface" table "$VRF_TABLE_CP"
        ;;
      *)
        $IP route add default via "$gw" dev "$iface" table "$VRF_TABLE_CP"
        ;;
    esac
  fi
done

if [ -n "${VRF_MAIN_DEFAULT:-}" ]; then
  $IP link add "$VRF_MAIN_DEFAULT_IFACE" type dummy
  $IP addr add "$VRF_MAIN_DEFAULT_ADDR" dev "$VRF_MAIN_DEFAULT_IFACE"
  $IP link set "$VRF_MAIN_DEFAULT_IFACE" up
  $IP route replace default via "$VRF_MAIN_DEFAULT_GW" dev "$VRF_MAIN_DEFAULT_IFACE" table main
  log "installed dead-end default route in table main via $VRF_MAIN_DEFAULT_IFACE"
fi

log "VRF setup done"
$IP link show type vrf || true
$IP route show table main || true
$IP route show table "$VRF_TABLE_UP" || true
$IP route show table "$VRF_TABLE_CP" || true

exec "$CORE_BIN" --config "$CORE_CONFIG"
