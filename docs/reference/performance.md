---
description: Reference for performance results - data plane throughput and latency, and PDU session support.
---

# Performance

This reference document contains performance test results of Ella Core, covering data plane throughput and latency as well as PDU session support.

## Results

### Throughput

The following table outlines the performance test results of Ella Core's data plane throughput:

| Uplink (Gbps) | Downlink (Gbps) |
| ------------- | --------------- |
| 5.47          | 1.77            |

### Latency (Round-trip)

The following table outlines the performance test results of Ella Core's data plane latency:

| Average (ms) | Best (ms) | Worst (ms) | Mean Deviation (ms) |
| ------------ | --------- | ---------- | ----------------------- |
| 1.401        | 1.049     | 1.512      | 0.082                   |

The value represents the round-trip-response times from the UE to the server and back.

### PDU Session Support

Ella Core can support up to **1000 subscribers** using a PDU session simultaneously. This was tested with **ueransim**,
using 10 simulated gNodeBs each handling 100 subscribers.

## Methodology

We performed performance tests with Ella Core running on a baremetal system with the following specifications:

- **System**: Protectli Vault Pro VP6670
- **OS**: Ubuntu 22.04 LTS
- **CPU**: 12th Gen Intel(R) Core(TM) i7-1255U
- **RAM**: 16GB
- **Disk**: 256GB NVMe SSD

<figure markdown="span">
  ![Connectivity](../images/performance_setup.svg){ width="800" }
  <figcaption>Performance Testing Environment</figcaption>
</figure>

### Throughput testing

We performed the throughput tests using [iPerf3](https://iperf.fr/).

Test parameters:

- **Version**: v3.16
- **Protocol**: TCP
- **Duration**: 30 seconds
- **Streams**: 1
- **MTU (upstream)**: 1424 bytes
- **MTU (downstream)**: 1416 bytes
- **Runs (average over)**: 5

### Latency testing

We performed latency tests using ping.

Test parameters:

- **Count**: 30
