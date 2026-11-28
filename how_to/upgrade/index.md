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

1. Stop Ella Core.

1. In the directory where you extracted Ella Core, replace the binary with the target release (replace `amd64` with `arm64` on arm64 hosts):

   ```
   curl -L https://github.com/ellanetworks/core/releases/download/<target-version>/ella-core-<target-version>-linux-amd64.tar.gz | tar xz --strip-components=1 ella-core-<target-version>-linux-amd64/core
   ```

1. Start Ella Core:

   ```
   sudo ./core --config core.yaml
   ```

In `docker-compose.yaml`, set the image tag to the target release:

```
image: ghcr.io/ellanetworks/ella-core:<target-version>
```

Recreate the container:

```
docker compose up -d
```

Upgrade the chart:

```
helm upgrade ella-core oci://ghcr.io/ellanetworks/charts/ella-core --version <target-version> --namespace ella -f values.yaml
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

Upgrade the chart. The pods keep running the current release until you restart them:

```
helm upgrade ella-core oci://ghcr.io/ellanetworks/charts/ella-core --version <target-version> --namespace ella -f values.yaml
```

Upgrade the followers first and the leader last. Repeat these steps for each node `<n>`:

1. Open the **Cluster** page on any healthy node other than the one you are about to upgrade.

1. Click **Drain** next to the node. Wait until its **Drain State** is `drained`.

1. Restart the node's pod:

   ```
   kubectl -n ella delete pod ella-core-<n>-0
   ```

1. On the **Cluster** page, wait for the node to return to **Healthy**.

1. Click **Resume** next to the node. Wait until its **Drain State** is `active`.

1. Wait for the pod to be ready:

   ```
   kubectl -n ella wait --for=condition=Ready pod/ella-core-<n>-0 --timeout=10m
   ```

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

Run these steps in the directory where you extracted Ella Core.

1. Stop Ella Core.

1. Replace the binary with the previous release:

   ```
   curl -L https://github.com/ellanetworks/core/releases/download/<previous-version>/ella-core-<previous-version>-linux-amd64.tar.gz | tar xz --strip-components=1 ella-core-<previous-version>-linux-amd64/core
   ```

1. Delete the database and copy the backup as `restore.bundle`:

   ```
   sudo rm -rf ella.db ella.db-wal ella.db-shm raft cluster-tls
   sudo cp backup.tar.gz restore.bundle
   ```

1. Start Ella Core:

   ```
   sudo ./core --config core.yaml
   ```

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

1. Roll the chart back to the previous release. This also scales Ella Core up:

   ```
   helm rollback ella-core --namespace ella
   ```

1. On every node, stop Ella Core and revert to the previous release:

   ```
   sudo snap stop --disable ella-core.cored
   sudo snap revert ella-core
   ```

1. Restore the backup with [Offline Restore](https://docs.ellanetworks.com/core/how_to/backup_and_restore/#offline-restore-disaster-recovery).
