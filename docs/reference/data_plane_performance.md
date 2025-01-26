---
description: Reference of the data plane performance results - Throughput and latency.
---

# Data Plane Performance

This reference document contains performance test results of Ella Core's data plane.

## Throughput

The following table outlines the performance test results of Ella Core's data plane throughput:

| Streams | Uplink (Gbps) | Downlink (Gbps) |
| ------- | ------------- | --------------- |
| 1       | 1.20          | 1.00            |
| 10      | 1.48          | 1.22            |
| 100     | 1.54          | 1.25            |

## Latency

The following table outlines the performance test results of Ella Core's data plane latency:

| Average (ms) | Best (ms) | Worst (ms) | Standard Deviation (ms) |
| ------------ | --------- | ---------- | ----------------------- |
| 1.4          | 0.8       | 4.0        | 0.6                     |

The value represents the round-trip-response times.

### Methodology

#### Environment

The tests were performed on a computer with the following specifications:

- **CPU**: Intel(R) Core(TM) Ultra 7 265K
- **RAM**: 64GB
- **Disk**: 1TB NVMe SSD

The same virtualized environment as outlined in the [Running an End-to-End 5G Network with Ella Core](../tutorials/running_end_to_end_network.md) tutorial was used. The iPerf3 server was running on the router virtual machine, and the iPerf3 client was running on the subscriber virtual machine.

!!! note
    The performance tests were performed on a virtualized environment. The results will likely improve in a bare-metal environment.

#### Throughput testing

The throughput tests were performed using [iPerf3](https://iperf.fr/) for 30 seconds.

#### Latency testing

The latency tests were performed using [mtr](https://manpages.ubuntu.com/manpages/focal/man8/mtr.8.html) in TCP mode with 60 report cycles.
