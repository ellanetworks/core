// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/integration/suites"
)

const releaseFixtureOutEnv = "RELEASE_FIXTURE_OUT"

func TestGenerateReleaseFixture(t *testing.T) {
	suites.Require(t, suites.RollingUpgrade)

	out := os.Getenv(releaseFixtureOutEnv)
	if out == "" {
		t.Skipf("set %s to the output archive, e.g. internal/db/testdata/releases/vX.Y.Z.tar.gz", releaseFixtureOutEnv)
	}

	beginHATest(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	dc := newStandaloneUpgradeDocker(t, ctx)

	node, _ := startStandaloneOnImage(t, ctx, dc, rollingTargetImage)

	seedUpgradeData(t, ctx, node)
	seedReleaseFeatures(t, ctx, node)

	setUpUpgradeTraffic(t, ctx, dc, haRollingComposeDir, []*client.Client{node})
	runUpgradeTraffic(t, ctx, dc, 1, "while generating the release fixture")

	waitForFlowReports(t, ctx, node)

	if err := dc.ComposeStopWithFile(ctx, haRollingComposeDir, ComposeFile(), standaloneUpgradeService); err != nil {
		t.Fatalf("stop node: %v", err)
	}

	container, err := dc.ResolveComposeContainer(ctx, "ha-rolling", standaloneUpgradeService)
	if err != nil {
		t.Fatalf("resolve node container: %v", err)
	}

	dataDir := filepath.Join(t.TempDir(), "data")

	if output, err := exec.CommandContext(ctx, "docker", "cp", container+":/data", dataDir).CombinedOutput(); err != nil {
		t.Fatalf("copy data directory: %v: %s", err, output)
	}

	if err := writeReleaseFixture(out, dataDir); err != nil {
		t.Fatalf("write %s: %v", out, err)
	}

	HALogf(t, "release fixture written to %s", out)
}

func seedReleaseFeatures(t *testing.T, ctx context.Context, c *client.Client) {
	t.Helper()

	localSwitch := true
	imsDN := true

	steps := []struct {
		name string
		run  func() error
	}{
		{"enable local switch on the seeded slice data network", func() error {
			return c.UpdateSliceDataNetwork(ctx, upgradeSeedName, upgradeSeedName, &client.UpdateSliceDataNetworkOptions{LocalSwitch: &localSwitch})
		}},
		{"create IMS data network", func() error {
			return c.CreateDataNetwork(ctx, &client.CreateDataNetworkOptions{Name: "ims", IPv4Pool: "10.89.0.0/24", DNS: "8.8.8.8", Mtu: 1400, IMS: &imsDN})
		}},
		{"serve the IMS data network on the slice", func() error {
			return serveDataNetworkOnSlice(ctx, c, upgradeSeedName, "ims")
		}},
		{"create IMS policy", func() error {
			return c.CreatePolicy(ctx, &client.CreatePolicyOptions{
				Name: "upgrade-seed-ims", ProfileName: upgradeSeedName, SliceName: upgradeSeedName, DataNetworkName: "ims",
				SessionAmbrUplink: "10 Mbps", SessionAmbrDownlink: "10 Mbps", Var5qi: 5, Arp: 1,
			})
		}},
		{"update IMS settings", func() error {
			return c.UpdateIMSSettings(ctx, &client.UpdateIMSSettingsOptions{
				PCSCFAddresses: []string{"10.89.0.250"}, HomeDomain: "ims.example.org", AdvertiseVoice: true,
			})
		}},
		{"create allowed IMS peer", func() error {
			_, err := c.CreateAllowedIMSPeer(ctx, &client.CreateAllowedIMSPeerOptions{Name: "cscf", Realm: "ims.example.org", Addresses: []string{"10.89.0.0/24"}})
			return err
		}},
		{"create IMS application server", func() error {
			_, err := c.CreateIMSApplicationServer(ctx, &client.CreateIMSApplicationServerOptions{
				Name: "mcptt", ServerName: "sip:mcptt.ims.example.org", PublicServiceIdentities: []string{"sip:!.*!@mcptt.ims.example.org"},
			})

			return err
		}},
		{"create IMS service profile", func() error {
			_, err := c.CreateIMSServiceProfile(ctx, &client.CreateIMSServiceProfileOptions{
				Name:                  "mcptt",
				InitialFilterCriteria: []client.IMSInitialFilterCriterion{{ApplicationServerName: "mcptt"}},
			})

			return err
		}},
		{"assign IMS service profile", func() error {
			mcptt := "mcptt"
			return c.SetProfileIMSServiceProfiles(ctx, upgradeSeedName, &client.ProfileIMSServiceProfiles{MCPTT: &mcptt})
		}},
		{"update SMS settings", func() error {
			return c.UpdateSMSSettings(ctx, &client.UpdateSMSSettingsOptions{ServingNodeNumber: "+15550001111"})
		}},
		{"create SMSC", func() error {
			_, err := c.CreateSMSC(ctx, &client.CreateSMSCOptions{
				Name: "smsc", ServiceCentres: []string{"+15550000000"},
				Nodes: []client.SMSCNodeOptions{
					{OriginHost: "smsc-1.example.org", Address: "192.0.2.10", Port: 3868},
					{OriginHost: "smsc-2.example.org", Address: "192.0.2.11", Port: 3868},
				},
			})

			return err
		}},
		{"create roaming partner", func() error {
			peer, err := c.CreateRoamingDiameterPeer(ctx, &client.RoamingDiameterPeerOptions{OriginHost: "hss.partner.example.org", Address: "192.0.2.20", Port: 3868})
			if err != nil {
				return err
			}

			_, err = c.CreateRoamingPartner(ctx, &client.RoamingPartnerOptions{
				Name: "partner", PLMNs: []client.RoamingPLMN{{Mcc: "208", Mnc: "93"}}, ProfileName: upgradeSeedName, DiameterPeerID: peer.ID,
			})

			return err
		}},
		{"disable NAT on the seeded slice data network", func() error {
			nat := false
			return c.UpdateSliceDataNetwork(ctx, upgradeSeedName, upgradeSeedName, &client.UpdateSliceDataNetworkOptions{NAT: &nat})
		}},
		{"create framed route", func() error {
			return c.CreateDataNetworkFramedRoute(ctx, upgradeSeedName, &client.CreateFramedRouteOptions{IMSI: upgradeSeedIMSI, IPv4: []string{"192.168.88.0/24"}})
		}},
		{"configure BGP", func() error {
			return c.UpdateBGPSettings(ctx, &client.UpdateBGPSettingsOptions{LocalAS: 64512, RouterID: "10.6.0.11", ListenAddress: ":179"})
		}},
		{"create BGP peer", func() error {
			return c.CreateBGPPeer(ctx, &client.CreateBGPPeerOptions{
				Address: "10.6.0.3", RemoteAS: 64513, HoldTime: 90, Description: "upgrade seed",
				ImportPrefixes: []client.BGPImportPrefix{{Prefix: "0.0.0.0/0"}},
			})
		}},
		{"create cell position", func() error {
			return c.CreateCellPosition(ctx, &client.CreateCellPositionOptions{RAT: "nr", Mcc: "001", Mnc: "01", CellIdentity: "000000010", Latitude: 45.5, Longitude: -73.6})
		}},
	}

	for _, s := range steps {
		if err := s.run(); err != nil {
			t.Fatalf("seed release features: %s: %v", s.name, err)
		}
	}
}

func waitForFlowReports(t *testing.T, ctx context.Context, c *client.Client) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Minute)

	for time.Now().Before(deadline) {
		reports, err := c.ListFlowReports(ctx, &client.ListFlowReportsParams{Page: 1, PerPage: 1})
		if err == nil && reports.TotalCount > 0 {
			return
		}

		time.Sleep(2 * time.Second)
	}

	t.Fatal("no flow reports were recorded")
}

func writeReleaseFixture(out, dataDir string) error {
	entries := map[string]string{"core.db": "ella.db", "node-id": "node-id"}

	f, err := os.Create(out) //nolint:gosec
	if err != nil {
		return err
	}

	defer func() { _ = f.Close() }()

	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)

	for name, src := range entries {
		if err := addFileToTar(tw, filepath.Join(dataDir, src), name); err != nil {
			return err
		}
	}

	err = filepath.WalkDir(filepath.Join(dataDir, "raft"), func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}

		rel, err := filepath.Rel(dataDir, path)
		if err != nil {
			return err
		}

		return addFileToTar(tw, path, filepath.ToSlash(rel))
	})
	if err != nil {
		return err
	}

	if err := tw.Close(); err != nil {
		return err
	}

	if err := gz.Close(); err != nil {
		return err
	}

	return f.Close()
}

func addFileToTar(tw *tar.Writer, path, name string) error {
	src, err := os.Open(path) //nolint:gosec
	if err != nil {
		return err
	}

	defer func() { _ = src.Close() }()

	info, err := src.Stat()
	if err != nil {
		return err
	}

	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: info.Size(), ModTime: info.ModTime()}); err != nil {
		return err
	}

	_, err = io.Copy(tw, src)

	return err
}
