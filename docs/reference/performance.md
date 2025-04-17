---
description: Reference for performance results - data plane throughput and latency, and PDU session support.
---

# Performance

This reference document contains performance test results of Ella Core, covering data plane throughput and latency as well as PDU session support.

## Data Plane

### Throughput

The following table outlines the performance test results of Ella Core's data plane throughput:

| Uplink (Gbps) | Downlink (Gbps) |
| ------------- | --------------- |
| 3.05          | 1.31            |

### Latency

The following table outlines the performance test results of Ella Core's data plane latency:

| Average (ms) | Best (ms) | Worst (ms) | Standard Deviation (ms) |
| ------------ | --------- | ---------- | ----------------------- |
| 1.702        | 0.903     | 2.338      | 0.269                   |

The value represents the round-trip-response times from the UE to the router and back.

## PDU Session Support

Ella Core can stand up **500 PDU sessions**, the maximum UERANSIM supports.

Further testing is required to determine the maximum number of PDU sessions Ella Core can support.

## Methodology

### Environment

We performed performance tests on a system with the following specifications:

- **CPU**: Intel(R) Core(TM) Ultra 7 265K
- **RAM**: 64GB
- **Disk**: 1TB NVMe SSD

We used the same virtualized environment outlined in the [Running an End-to-End 5G Network with Ella Core](../tutorials/end_to_end_network.md) tutorial, with the iPerf3 server running on the router virtual machine, and the iPerf3 client running on the radio virtual machine. We used [Ella Core Tester](https://github.com/ellanetworks/core-tester) as the UE and gNB simulator.

!!! note
    We performed the performance tests in a virtualized environment. The results will likely improve in a bare-metal environment.

#### Throughput testing

We performed the throughput tests using [iPerf3](https://iperf.fr/).

Test parameters:

- **Version**: v3.16
- **Protocol**: TCP
- **Duration**: 30 seconds
- **Streams**: 1
- **MTU (upstream)**: 1424 bytes
- **MTU (downstream)**: 1416 bytes
- **Runs (average over)**: 5 

#### Latency testing

We performed latency tests using ping.

Test parameters:

- **Count**: 30
