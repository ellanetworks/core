// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/integration/suites"
)

const standaloneUpgradeService = "ella-core-1"

const (
	latestSchemaWriteTimeout = 60 * time.Second
)

func TestIntegrationStandaloneUpgradeFromRelease(t *testing.T) {
	suites.Require(t, suites.RollingUpgrade)

	beginHATest(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	if err := ensureRollingImages(ctx, t); err != nil {
		t.Fatalf("rolling-upgrade images unavailable: %v", err)
	}

	dc := newStandaloneUpgradeDocker(t, ctx)

	node, baseline := startStandaloneOnImage(t, ctx, dc, rollingBaselineImage)

	releaseIMSIs := createStandaloneSubscribers(t, ctx, node, "001019756160000", 10)

	backupPath := filepath.Join(t.TempDir(), "release-backup.tar.gz")

	if err := node.CreateBackup(ctx, &client.CreateBackupParams{Path: backupPath}); err != nil {
		t.Fatalf("back up the release node: %v", err)
	}

	swapNodeImage(t, ctx, dc, 1, rollingTargetImage)

	target := waitForUpgradedStandalone(t, ctx, node, baseline)

	assertSubscribersPresent(t, ctx, node, releaseIMSIs, "after the upgrade")

	upgradedIMSIs := createStandaloneSubscribers(t, ctx, node, "001019756161000", 1)

	for restart := 1; restart <= 2; restart++ {
		restartStandalone(t, ctx, dc, node)

		assertSubscribersPresent(t, ctx, node, releaseIMSIs, fmt.Sprintf("after restart %d", restart))
		assertSubscribersPresent(t, ctx, node, upgradedIMSIs, fmt.Sprintf("after restart %d", restart))
	}

	if err := node.RestoreBackup(ctx, &client.RestoreBackupParams{Path: backupPath}); err != nil {
		t.Fatalf("restore the release backup on the upgraded node: %v", err)
	}

	if err := waitForNodeReady(ctx, node); err != nil {
		t.Fatalf("node not ready after restoring the release backup: %v", err)
	}

	assertSubscribersPresent(t, ctx, node, releaseIMSIs, "after restoring the release backup")
	assertSubscribersAbsent(t, ctx, node, upgradedIMSIs, "after restoring the release backup")

	if baseline.SchemaVersion < target.SchemaVersion {
		HALogf(t, "restored a schema-%d backup on a schema-%d build; expecting a Raft migration without a restart",
			baseline.SchemaVersion, target.SchemaVersion)
	}

	createSubscriberWhenSchemaCurrent(t, ctx, node, "001019756162000")

	restartStandalone(t, ctx, dc, node)

	assertSubscribersPresent(t, ctx, node, releaseIMSIs, "after restarting the restored node")
}

func TestIntegrationStandaloneDisasterRecoveryFromRelease(t *testing.T) {
	suites.Require(t, suites.RollingUpgrade)

	beginHATest(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	if err := ensureRollingImages(ctx, t); err != nil {
		t.Fatalf("rolling-upgrade images unavailable: %v", err)
	}

	dc := newStandaloneUpgradeDocker(t, ctx)

	node, baseline := startStandaloneOnImage(t, ctx, dc, rollingBaselineImage)

	releaseIMSIs := createStandaloneSubscribers(t, ctx, node, "001019756170000", 5)

	backupPath := filepath.Join(t.TempDir(), "release-backup.tar.gz")

	if err := node.CreateBackup(ctx, &client.CreateBackupParams{Path: backupPath}); err != nil {
		t.Fatalf("back up the release node: %v", err)
	}

	dc.ComposeDownWithFile(ctx, haRollingComposeDir, ComposeFile())

	t.Setenv("ELLA_CORE_1_IMAGE", rollingTargetImage)
	t.Setenv("ELLA_CORE_2_IMAGE", rollingTargetImage)

	if err := writeStandaloneNodeConfig(haRollingComposeDir); err != nil {
		t.Fatalf("write recovered standalone config: %v", err)
	}

	if err := dc.ComposeCreateWithFile(ctx, haRollingComposeDir, ComposeFile(), standaloneUpgradeService); err != nil {
		t.Fatalf("create recovered node: %v", err)
	}

	container, err := dc.ResolveComposeContainer(ctx, "ha-rolling", standaloneUpgradeService)
	if err != nil {
		t.Fatalf("resolve recovered container: %v", err)
	}

	if err := dc.CopyFileToContainer(ctx, container, backupPath, "/data/restore.bundle"); err != nil {
		t.Fatalf("copy restore bundle: %v", err)
	}

	if err := dc.ComposeStartWithFile(ctx, haRollingComposeDir, ComposeFile(), standaloneUpgradeService); err != nil {
		t.Fatalf("start recovered node: %v", err)
	}

	waitForUpgradedStandalone(t, ctx, node, baseline)

	status := mustReadStatus(ctx, t, node)
	if !status.Initialized {
		t.Fatal("recovered node is not initialized; restore.bundle was not applied")
	}

	assertSubscribersPresent(t, ctx, node, releaseIMSIs, "after disaster recovery")

	createSubscriberWhenSchemaCurrent(t, ctx, node, "001019756171000")

	if err := dc.ComposeStopWithFile(ctx, haRollingComposeDir, ComposeFile(), standaloneUpgradeService); err != nil {
		t.Fatalf("stop recovered node: %v", err)
	}

	if err := writeNodeConfig(haRollingComposeDir, 1, nil, "", ""); err != nil {
		t.Fatalf("write cluster config for the recovered node: %v", err)
	}

	if err := dc.ComposeStartWithFile(ctx, haRollingComposeDir, ComposeFile(), standaloneUpgradeService); err != nil {
		t.Fatalf("restart recovered node as cluster seed: %v", err)
	}

	if err := waitForNodeReady(ctx, node); err != nil {
		t.Fatalf("recovered node never became ready as a cluster seed: %v", err)
	}

	peers := []string{ClusterAddressWithPort(1, 7000), ClusterAddressWithPort(2, 7000)}

	if err := stageAndStartJoiner(ctx, dc, node, haRollingComposeDir, "ella-core-2", 2, peers, ""); err != nil {
		t.Fatalf("join node 2 to the recovered seed: %v", err)
	}

	joiner, err := newInsecureClient(APIAddressForCluster(2))
	if err != nil {
		t.Fatalf("client for node 2: %v", err)
	}

	joiner.SetToken(node.GetToken())

	clients := []*client.Client{node, joiner}

	t.Cleanup(func() {
		dumpClusterDiagnostics(t, context.Background(), dc, haRollingComposeDir, haNodeServices[:2], clients)
	})

	if err := waitForClusterReady(ctx, clients); err != nil {
		t.Fatalf("two-node cluster grown from the recovered seed never became ready: %v", err)
	}

	if err := waitForAllNodesReady(ctx, clients); err != nil {
		t.Fatalf("nodes not ready: %v", err)
	}

	assertSubscribersPresent(t, ctx, joiner, releaseIMSIs, "on the joiner")
}

func newStandaloneUpgradeDocker(t *testing.T, ctx context.Context) *DockerClient {
	t.Helper()

	dc, err := NewDockerClient()
	if err != nil {
		t.Fatalf("docker client: %v", err)
	}

	t.Cleanup(func() { _ = dc.Close() })

	dc.ComposeCleanup(ctx)

	t.Cleanup(func() {
		dc.ComposeDownWithFile(context.Background(), haRollingComposeDir, ComposeFile())
	})

	return dc
}

func startStandaloneOnImage(t *testing.T, ctx context.Context, dc *DockerClient, image string) (*client.Client, *client.Status) {
	t.Helper()

	t.Setenv("ELLA_CORE_1_IMAGE", image)

	if err := writeStandaloneNodeConfig(haRollingComposeDir); err != nil {
		t.Fatalf("write standalone config: %v", err)
	}

	if err := dc.ComposeUpServicesWithFile(ctx, haRollingComposeDir, ComposeFile(), standaloneUpgradeService); err != nil {
		t.Fatalf("start standalone node on %s: %v", image, err)
	}

	node, err := newInsecureClient(APIAddressForCluster(1))
	if err != nil {
		t.Fatalf("client for node 1: %v", err)
	}

	t.Cleanup(func() {
		dumpClusterDiagnostics(t, context.Background(), dc, haRollingComposeDir, haNodeServices[:1], []*client.Client{node})
	})

	if err := waitForNodeReady(ctx, node); err != nil {
		t.Fatalf("standalone node on %s never became ready: %v", image, err)
	}

	token, err := initializeAndGetAdminToken(ctx, node)
	if err != nil {
		t.Fatalf("initialize standalone node: %v", err)
	}

	node.SetToken(token)

	status := mustReadStatus(ctx, t, node)
	if status.Cluster.Enabled {
		t.Fatalf("node on %s must start standalone", image)
	}

	HALogf(t, "standalone on %s: version %s, revision %s, schema %d",
		image, status.Version, status.Revision, status.SchemaVersion)

	return node, status
}

func waitForUpgradedStandalone(t *testing.T, ctx context.Context, node *client.Client, baseline *client.Status) *client.Status {
	t.Helper()

	if err := waitForNodeReady(ctx, node); err != nil {
		t.Fatalf("node not ready on %s: %v", rollingTargetImage, err)
	}

	target := mustReadStatus(ctx, t, node)

	if target.Revision == "" || target.Revision == baseline.Revision {
		t.Fatalf("target revision %q must be set and differ from the release's %q; rebuild %s from this tree",
			target.Revision, baseline.Revision, rollingTargetImage)
	}

	if target.SchemaVersion < baseline.SchemaVersion {
		t.Fatalf("target schema %d < release schema %d: this build is older than %s; set ROLLING_BASELINE_VERSION to a release this build supersedes",
			target.SchemaVersion, baseline.SchemaVersion, rollingBaselineImage)
	}

	if target.Cluster.Enabled {
		t.Fatal("the upgraded node must stay standalone")
	}

	HALogf(t, "upgraded to %s: version %s, revision %s, schema %d (release schema %d)",
		rollingTargetImage, target.Version, target.Revision, target.SchemaVersion, baseline.SchemaVersion)

	return target
}

func standaloneSubscriber(imsi string) *client.CreateSubscriberOptions {
	return &client.CreateSubscriberOptions{
		Imsi:           imsi,
		Key:            "0eefb0893e6f1c2855a3a244c6db1277",
		OPc:            "98da19bbc55e2a5b53857d10557b1d26",
		SequenceNumber: "000000000022",
		ProfileName:    "default",
	}
}

func createStandaloneSubscribers(t *testing.T, ctx context.Context, c *client.Client, base string, n int) []string {
	t.Helper()

	imsis := make([]string, 0, n)

	for i := 0; i < n; i++ {
		imsi, err := offsetIMSI15(base, i)
		if err != nil {
			t.Fatalf("compute IMSI: %v", err)
		}

		if err := c.CreateSubscriber(ctx, standaloneSubscriber(imsi)); err != nil {
			t.Fatalf("create subscriber %s on a ready node: %v", imsi, err)
		}

		imsis = append(imsis, imsi)
	}

	return imsis
}

func createSubscriberWhenSchemaCurrent(t *testing.T, ctx context.Context, c *client.Client, imsi string) {
	t.Helper()

	deadline := time.Now().Add(latestSchemaWriteTimeout)

	var lastErr error

	for time.Now().Before(deadline) {
		lastErr = c.CreateSubscriber(ctx, standaloneSubscriber(imsi))
		if lastErr == nil {
			return
		}

		if !isTransientWriteError(lastErr) {
			t.Fatalf("create subscriber %s: %v", imsi, lastErr)
		}

		time.Sleep(time.Second)
	}

	t.Fatalf("writes needing the latest schema still refused after %s: %v", latestSchemaWriteTimeout, lastErr)
}

func restartStandalone(t *testing.T, ctx context.Context, dc *DockerClient, node *client.Client) {
	t.Helper()

	if err := dc.ComposeStopWithFile(ctx, haRollingComposeDir, ComposeFile(), standaloneUpgradeService); err != nil {
		t.Fatalf("stop standalone node: %v", err)
	}

	if err := dc.ComposeStartWithFile(ctx, haRollingComposeDir, ComposeFile(), standaloneUpgradeService); err != nil {
		t.Fatalf("start standalone node: %v", err)
	}

	if err := waitForNodeReady(ctx, node); err != nil {
		t.Fatalf("standalone node not ready after restart: %v", err)
	}
}

func assertSubscribersPresent(t *testing.T, ctx context.Context, c *client.Client, imsis []string, when string) {
	t.Helper()

	for _, imsi := range imsis {
		sub, err := c.GetSubscriber(ctx, &client.GetSubscriberOptions{ID: imsi})
		if err != nil {
			t.Fatalf("subscriber %s missing %s: %v", imsi, when, err)
		}

		if sub.Imsi != imsi {
			t.Fatalf("subscriber %s read back as %q %s", imsi, sub.Imsi, when)
		}
	}
}

func assertSubscribersAbsent(t *testing.T, ctx context.Context, c *client.Client, imsis []string, when string) {
	t.Helper()

	for _, imsi := range imsis {
		if _, err := c.GetSubscriber(ctx, &client.GetSubscriberOptions{ID: imsi}); err == nil {
			t.Fatalf("subscriber %s still present %s; the restore did not replace the data", imsi, when)
		}
	}
}
