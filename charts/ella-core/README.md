# Ella Core Helm Chart

[Ella Core](https://docs.ellanetworks.com/core/) is a 5G and 4G mobile core network. This chart deploys Ella Core as a standalone node or as a three-node high-availability cluster.

## Prerequisites

The cluster must meet the [Kubernetes requirements](https://docs.ellanetworks.com/core/reference/system_reqs/#kubernetes).

## Install

```bash
kubectl create namespace ella
kubectl label namespace ella pod-security.kubernetes.io/enforce=privileged pod-security.kubernetes.io/warn=privileged
helm install ella-core oci://ghcr.io/ellanetworks/charts/ella-core --namespace ella -f values.yaml
```

A standalone node:

```yaml
nodes:
  - ranAddress: 192.168.252.3/24
ran:
  networkAttachment:
    master: eth1
```

A high-availability cluster:

```yaml
nodes:
  - ranAddress: 192.168.252.3/24
  - ranAddress: 192.168.252.4/24
  - ranAddress: 192.168.252.5/24
ran:
  networkAttachment:
    master: eth1
```

See [Install](https://docs.ellanetworks.com/core/how_to/install/) and [Deploy a High Availability Cluster](https://docs.ellanetworks.com/core/how_to/deploy_ha_cluster/).

## Values

| Key | Default | Description |
|-----|---------|-------------|
| `nodes` | one node | One entry per Ella Core node. One entry deploys a standalone node. Three entries deploy a high-availability cluster, with one node per Kubernetes node. |
| `nodes[].ranAddress` | `192.168.252.3/24` | N2/N3 address of the node, in CIDR notation. |
| `nodes[].nodeSelector` | | Node selector for this node only. Overrides `nodeSelector`. |
| `ran.interface` | `n3` | Name of the N2/N3 interface in the pod. |
| `ran.networkAttachment.create` | `true` | Create a macvlan NetworkAttachmentDefinition for the N2/N3 interface. Set to `false` to use your own. |
| `ran.networkAttachment.name` | `<release>-ran` | Name of the NetworkAttachmentDefinition. |
| `ran.networkAttachment.master` | `eth1` | Host interface that connects to the radios. |
| `ran.networkAttachment.mode` | `bridge` | macvlan mode. |
| `ran.networkAttachment.routes` | `[]` | Routes through the N2/N3 interface, e.g. `{dst: 10.20.0.0/16, gw: 192.168.252.1}` for radios behind a router. |
| `config` | logging to stdout, `tcx` datapath | [Configuration file](https://docs.ellanetworks.com/core/reference/config_file/) content, merged over the database, interface, API and cluster settings that the chart sets. |
| `image.repository` | `ghcr.io/ellanetworks/ella-core` | Image repository. |
| `image.tag` | chart `appVersion` | Image tag. |
| `image.digest` | | Image digest. Takes precedence over `image.tag`. |
| `image.pullPolicy` | `IfNotPresent` | Image pull policy. |
| `imagePullSecrets` | `[]` | Image pull secrets. |
| `service.type` | `ClusterIP` | Type of the API service. |
| `service.port` | `5002` | Port of the API service. |
| `service.nodePort` | | Node port, when `service.type` is `NodePort`. |
| `service.annotations` | `{}` | Annotations of the API service. |
| `api.tls.secretName` | | `kubernetes.io/tls` Secret for HTTPS served by the pod itself. The probes and `helm test` then use HTTPS. With an Ingress, set the controller's annotation for HTTPS to the backend. |
| `ingress.enabled` | `false` | Create an Ingress that exposes the API and UI. |
| `ingress.className` | | IngressClass name. Empty uses the cluster default. |
| `ingress.host` | | Host name of the API and UI. Required when the Ingress is enabled. |
| `ingress.annotations` | `{}` | Ingress annotations, e.g. for cert-manager. |
| `ingress.tls` | `[]` | Ingress TLS entries, each with `secretName` and `hosts`. |
| `persistence.size` | `10Gi` | Size of each node's data volume. |
| `persistence.storageClassName` | default StorageClass | StorageClass of the data volumes. |
| `resources` | 1 CPU, 1 GiB requested; 4 CPU, 8 GiB limit | Container resources. |
| `serviceAccount.create` | `true` | Create a service account. |
| `serviceAccount.name` | `<release>` | Service account name. |
| `serviceAccount.annotations` | `{}` | Service account annotations. |
| `openshift.securityContextConstraints.create` | `auto` | Create a SecurityContextConstraints that grants the pod its capabilities. `auto` creates it on OpenShift only. |
| `seLinuxOptions` | `type: spc_t` on OpenShift | SELinux options of the container. Loading eBPF programs on an SELinux-enforcing node needs a type such as `spc_t`. |
| `serviceMonitor.enabled` | `false` | Create a Prometheus Operator ServiceMonitor for `/api/v1/metrics`. |
| `serviceMonitor.interval` | `30s` | Scrape interval. |
| `serviceMonitor.labels` | `{}` | Extra ServiceMonitor labels. |
| `clusterDomain` | `cluster.local` | Kubernetes cluster domain, used in the cluster addresses of high-availability nodes. |
| `nodeSelector` | `{}` | Node selector. |
| `tolerations` | `[]` | Tolerations. |
| `affinity` | one node per Kubernetes node in high availability | Affinity. Replaces the default anti-affinity. |
| `priorityClassName` | | Priority class. |
| `podAnnotations` | `{}` | Extra pod annotations. |
| `podLabels` | `{}` | Extra pod labels. |
| `nameOverride` | | Overrides the chart name. |
| `fullnameOverride` | | Overrides the resource name prefix. |
| `tests.image.repository` | `registry.access.redhat.com/ubi10/ubi-minimal` | Image of the `helm test` pod. |
| `tests.image.tag` | `10.2` | Tag of the `helm test` image. |
