import sys
from scapy.all import (
    conf,
    sr1,
    ICMP,
    icmptypes,
    IP,
    IPerror,
    TCP,
    UDP,
    UDPerror,
)

DEST = "192.168.250.1"
SPORT = 4242
DPORT = 34242


def main():
    exit_code = 0
    conf.route.add(host=DEST, dev=sys.argv[1])
    try:
        test_tcp()
    except Exception as e:
        print(f"TCP test failed with {e}")
        exit_code = 1
    try:
        test_udp()
    except Exception as e:
        print(f"UDP test failed with {e}")
        exit_code = 1
    try:
        test_icmp_echo_request()
    except Exception as e:
        print(f"ICMP Echo test failed with {e}")
        exit_code = 1
    try:
        test_icmp_timestamp_request()
    except Exception as e:
        print(f"ICMP Timestamp test failed with {e}")
        exit_code = 1
    try:
        test_udp_error()
    except Exception as e:
        print(f"UDP Error test failed with {e}")
        exit_code = 1
    sys.exit(exit_code)


def test_tcp():
    packet = IP(dst=DEST) / TCP(sport=SPORT, dport=DPORT)

    resp = sr1(packet, timeout=10)
    assert resp is not None
    assert resp[IP].src == DEST
    assert resp[TCP].sport == DPORT
    assert resp[TCP].dport == SPORT
    assert resp[TCP].flags == "SA"  # Syn-Ack


def test_udp():
    packet = IP(dst=DEST) / UDP(sport=SPORT, dport=DPORT)

    resp = sr1(packet, timeout=10)
    assert resp is not None
    assert resp[IP].src == DEST
    assert resp[UDP].sport == DPORT
    assert resp[UDP].dport == SPORT


def test_icmp_echo_request():
    ECHO_REQUEST = next(key for key in icmptypes if icmptypes[key] == "echo-request")
    packet = IP(dst=DEST) / ICMP(type=ECHO_REQUEST, id=SPORT)

    resp = sr1(packet, timeout=10)
    assert resp is not None
    assert resp[IP].src == DEST
    assert icmptypes[resp[ICMP].type] == "echo-reply"


def test_icmp_timestamp_request():
    TIMESTAMP_REQUEST = next(key for key in icmptypes if icmptypes[key] == "timestamp-request")
    packet = IP(dst=DEST) / ICMP(type=TIMESTAMP_REQUEST, id=SPORT)

    resp = sr1(packet, timeout=10)
    assert resp is not None
    assert resp[IP].src == DEST
    assert icmptypes[resp[ICMP].type] == "timestamp-reply"


def test_udp_error():
    packet = IP(dst=DEST) / UDP(sport=SPORT, dport=60000)

    resp = sr1(packet, timeout=10)
    assert resp is not None
    assert resp[IP].src == DEST
    assert icmptypes[resp[ICMP].type] == "dest-unreach"
    assert resp[IPerror].dst == DEST
    assert resp[UDPerror].sport == SPORT
    assert resp[UDPerror].dport == 60000


if __name__ == "__main__":
    main()
