---
description: Reference for performance results - data plane throughput and latency, and PDU session support.
---

# Performance

This reference document contains performance test results of Ella Core, covering data plane throughput and latency as well as PDU session support.

## Data Plane

### Throughput

The following table outlines the performance test results of Ella Core's data plane throughput:

| Streams | Uplink (Gbps) | Downlink (Gbps) |
| ------- | ------------- | --------------- |
| 1       | 1.20          | 1.00            |
| 10      | 1.48          | 1.22            |
| 100     | 1.54          | 1.25            |

### Latency

The following table outlines the performance test results of Ella Core's data plane latency:

| Average (ms) | Best (ms) | Worst (ms) | Standard Deviation (ms) |
| ------------ | --------- | ---------- | ----------------------- |
| 1.4          | 0.8       | 4.0        | 0.6                     |

The value represents the round-trip-response times.

## PDU Session Support

Ella Core can stand up **500 PDU sessions**, the maximum UERANSIM supports.

Further testing is required to determine the maximum number of PDU sessions Ella Core can support.

## Methodology

### Environment

We performed performance tests on a system with the following specifications:

- **CPU**: Intel(R) Core(TM) Ultra 7 265K
- **RAM**: 64GB
- **Disk**: 1TB NVMe SSD

We used the same virtualized environment outlined in the [Running an End-to-End 5G Network with Ella Core](../tutorials/end_to_end_network_snap.md) tutorial, with the iPerf3 server running on the router virtual machine, and the iPerf3 client running on the radio virtual machine.

!!! note
    We performed the performance tests in a virtualized environment. The results will likely improve in a bare-metal environment.

#### Throughput testing

We performed the throughput tests using [iPerf3](https://iperf.fr/).

Test parameters:

- **Version**: v3.16
- **Protocol**: TCP
- **Duration**: 30 seconds

#### Latency testing

We performed latency tests using [mtr](https://manpages.ubuntu.com/manpages/focal/man8/mtr.8.html).

Test parameters:

- **Version**: v0.95
- **Protocol**: TCP
- **Report Cycles**: 60
