# System Requirements

Ella Core runs on major Linux distributions including **Ubuntu**, **Ubuntu Core**, **Debian**, and **Arch Linux** (kernel 6.8 or later, with the `sctp` module available). It supports **amd64** and **arm64** architectures.

|             | 1,000 sessions   | 100,000 sessions |
| ----------- | ---------------- | ---------------- |
| **CPU**     | 1 core           | 4 cores          |
| **Memory**  | 1 GB RAM         | 8 GB RAM         |
| **Storage** | 10 GB disk space | 1 TB disk space  |

See [connectivity reference](https://docs.ellanetworks.com/core/reference/connectivity/index.md) for network interface requirements and configuration.

## Kubernetes

Ella Core should work on most Kubernetes distributions. It has been validated on **EKS**, **GKE**, **OpenShift**, **RKE2**, and **kind**.

| Requirement           | Value                                                                                                                                                                                                                                                                                                                                          |
| --------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Nodes**             | Meet the host requirements above                                                                                                                                                                                                                                                                                                               |
| **RAN network**       | Radios reach the pod's N2/N3 address directly. The Helm chart uses [Multus](https://github.com/k8snetworkplumbingwg/multus-cni) with the `macvlan`, `static`, and `tuning` [CNI plugins](https://github.com/containernetworking/plugins). The `tuning` plugin must be allowed to set `net.ipv4.ip_forward` and `net.ipv6.conf.all.forwarding`. |
| **Pod Security**      | `privileged`                                                                                                                                                                                                                                                                                                                                   |
| **Capabilities**      | `NET_ADMIN`, `NET_RAW`, `NET_BIND_SERVICE`, `BPF`, `PERFMON`, `SYS_RESOURCE`                                                                                                                                                                                                                                                                   |
| **Storage**           | A default StorageClass with `ReadWriteOnce` volumes, one per pod                                                                                                                                                                                                                                                                               |
| **High availability** | Three nodes                                                                                                                                                                                                                                                                                                                                    |
