# Upgrade

## Prerequisites

- Ella Core installed with [Install](https://docs.ellanetworks.com/core/how_to/install/index.md) or [Deploy a High Availability Cluster](https://docs.ellanetworks.com/core/how_to/deploy_ha_cluster/index.md).
- The target release is the next minor release after the running one.

## 1. Review the release notes

Read the [release notes](https://docs.ellanetworks.com/core/reference/releases/index.md) of the target release. Apply any action listed under **Breaking changes**.

## 2. Back up

Create a backup with [Backup and Restore](https://docs.ellanetworks.com/core/how_to/backup_and_restore/#backup).

## 3. Upgrade

```
sudo snap refresh ella-core
```

In `docker-compose.yaml`, set the image tag to the target release:

```
image: ghcr.io/ellanetworks/ella-core:<target-version>
```

Recreate the container:

```
docker compose up -d
```

Download the manifests of the target release:

```
git clone --depth 1 --branch <target-version> https://github.com/ellanetworks/core.git core-<target-version>
```

Reapply your changes to `core-<target-version>/k8s/core-ran-nad.yaml` and `core-<target-version>/k8s/core-configmap.yaml`.

Apply the manifests:

```
kubectl apply -k core-<target-version>/k8s
```

Wait for the rollout to complete:

```
kubectl -n ella rollout status statefulset/ella-core
```

Upgrade the followers first and the leader last. Repeat these steps for each node:

1. Open the **Cluster** page on any healthy node other than the one you are about to upgrade.

1. Click **Drain** next to the node. Wait until its **Drain State** is `drained`.

1. On that host, refresh the snap:

   ```
   sudo snap refresh ella-core
   ```

1. On the **Cluster** page, wait for the node to return to **Healthy**.

1. Click **Resume** next to the node. Wait until its **Drain State** is `active`.

## 4. Verify

```
curl -k <ella-core-api-url>/api/v1/status
```

Confirm `version` is the target release and `ready` is `true`.

On the **Cluster** page, confirm:

- Every node's **Version** is the target release.
- Every node is **Healthy** and its **Drain State** is `active`.
- **Schema** shows a single version.

## Roll back

1. Stop Ella Core:

   ```
   sudo snap stop --disable ella-core.cored
   ```

1. Revert to the previous release:

   ```
   sudo snap revert ella-core
   ```

1. Restore the backup with [Offline Restore](https://docs.ellanetworks.com/core/how_to/backup_and_restore/#offline-restore-disaster-recovery).

1. In `docker-compose.yaml`, set the image tag back to the previous release.

1. Stop the container:

   ```
   docker compose stop
   ```

1. Delete the database from the data volume:

   ```
   docker run --rm -v <project>_ella-data:/data alpine sh -c 'rm -rf /data/ella.db /data/ella.db-wal /data/ella.db-shm /data/raft /data/cluster-tls'
   ```

1. Copy the backup into the data volume as `restore.bundle`:

   ```
   docker compose cp backup.tar.gz ella-core:/data/restore.bundle
   ```

1. Recreate the container:

   ```
   docker compose up -d
   ```

1. Scale Ella Core down:

   ```
   kubectl -n ella scale statefulset/ella-core --replicas=0
   ```

1. Start a pod that mounts the data volume:

   ```
   kubectl -n ella run restore --image=alpine --restart=Never \
     --overrides='{"spec":{"containers":[{"name":"restore","image":"alpine","command":["sleep","3600"],"volumeMounts":[{"name":"data","mountPath":"/data"}]}],"volumes":[{"name":"data","persistentVolumeClaim":{"claimName":"data-ella-core-0"}}]}}'
   kubectl -n ella wait --for=condition=Ready pod/restore
   ```

1. Delete the database and copy the backup as `restore.bundle`:

   ```
   kubectl -n ella exec restore -- sh -c 'rm -rf /data/ella.db /data/ella.db-wal /data/ella.db-shm /data/raft /data/cluster-tls'
   kubectl -n ella cp backup.tar.gz restore:/data/restore.bundle
   kubectl -n ella delete pod restore
   ```

1. Download the manifests of the previous release:

   ```
   git clone --depth 1 --branch <previous-version> https://github.com/ellanetworks/core.git core-<previous-version>
   ```

1. Reapply your changes to `core-<previous-version>/k8s/core-ran-nad.yaml` and `core-<previous-version>/k8s/core-configmap.yaml`.

1. Apply the manifests:

   ```
   kubectl apply -k core-<previous-version>/k8s
   ```

1. Scale Ella Core up:

   ```
   kubectl -n ella scale statefulset/ella-core --replicas=1
   ```

1. On every node, stop Ella Core and revert to the previous release:

   ```
   sudo snap stop --disable ella-core.cored
   sudo snap revert ella-core
   ```

1. Restore the backup with [Offline Restore](https://docs.ellanetworks.com/core/how_to/backup_and_restore/#offline-restore-disaster-recovery).
