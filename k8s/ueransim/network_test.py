import argparse
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

SPORT = 4242
DPORT = 34242
CLOSED_DPORT = 60000


def parse_arguments():
    parser = argparse.ArgumentParser(description="Network scenarios tester")
    parser.add_argument(
        "--dev",
        type=str,
        required=True,
        help="NIC to use for testing",
    )
    parser.add_argument(
        "--dest",
        type=str,
        required=True,
        help="IPv4 address of the destination",
    )
    parser.add_argument(
        "--sport",
        type=int,
        default=SPORT,
        help="NIC to use for testing",
    )
    parser.add_argument(
        "--dport",
        type=int,
        default=DPORT,
        help="NIC to use for testing",
    )
    return parser.parse_args()


def main():
    exit_code = 0
    args = parse_arguments()
    conf.route.add(host=args.dest, dev=args.dev)
    try:
        test_tcp(args.dest, args.sport, args.dport)
    except Exception as e:
        print(f"TCP test failed with {e}")
        exit_code = 1
    try:
        test_udp(args.dest, args.sport, args.dport)
    except Exception as e:
        print(f"UDP test failed with {e}")
        exit_code = 1
    try:
        test_icmp_echo_request(args.dest, args.sport, args.dport)
    except Exception as e:
        print(f"ICMP Echo test failed with {e}")
        exit_code = 1
    try:
        test_icmp_timestamp_request(args.dest, args.sport, args.dport)
    except Exception as e:
        print(f"ICMP Timestamp test failed with {e}")
        exit_code = 1
    try:
        test_udp_error(args.dest, args.sport, args.dport)
    except Exception as e:
        print(f"UDP Error test failed with {e}")
        exit_code = 1
    sys.exit(exit_code)


def test_tcp(dest, sport, dport):
    packet = IP(dst=dest) / TCP(sport=sport, dport=dport)

    resp = sr1(packet, timeout=10)
    assert resp is not None
    assert resp[IP].src == dest
    assert resp[TCP].sport == dport
    assert resp[TCP].dport == sport
    assert resp[TCP].flags == "SA"  # Syn-Ack


def test_udp(dest, sport, dport):
    packet = IP(dst=dest) / UDP(sport=sport, dport=dport)

    resp = sr1(packet, timeout=10)
    assert resp is not None
    assert resp[IP].src == dest
    assert resp[UDP].sport == dport
    assert resp[UDP].dport == sport


def test_icmp_echo_request(dest, sport, dport):
    ECHO_REQUEST = next(
        key for key in icmptypes if icmptypes[key] == "echo-request"
    )
    packet = IP(dst=dest) / ICMP(type=ECHO_REQUEST, id=sport)

    resp = sr1(packet, timeout=10)
    assert resp is not None
    assert resp[IP].src == dest
    assert icmptypes[resp[ICMP].type] == "echo-reply"


def test_icmp_timestamp_request(dest, sport, dport):
    TIMESTAMP_REQUEST = next(
        key for key in icmptypes if icmptypes[key] == "timestamp-request"
    )
    packet = IP(dst=dest) / ICMP(type=TIMESTAMP_REQUEST, id=sport)

    resp = sr1(packet, timeout=10)
    assert resp is not None
    assert resp[IP].src == dest
    assert icmptypes[resp[ICMP].type] == "timestamp-reply"


def test_udp_error(dest, sport, dport):
    packet = IP(dst=dest) / UDP(sport=sport, dport=CLOSED_DPORT)

    resp = sr1(packet, timeout=10)
    assert resp is not None
    assert resp[IP].src == dest
    assert icmptypes[resp[ICMP].type] == "dest-unreach"
    assert resp[IPerror].dst == dest
    assert resp[UDPerror].sport == sport
    assert resp[UDPerror].dport == CLOSED_DPORT


if __name__ == "__main__":
    main()
