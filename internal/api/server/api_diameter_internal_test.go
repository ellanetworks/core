// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/diameternode"
)

type fakeDiameterNode struct {
	identity    diameternode.Identity
	identityErr error
	peers       []diameternode.PeerStatus
}

func (f fakeDiameterNode) Identity(context.Context) (diameternode.Identity, error) {
	return f.identity, f.identityErr
}

func (f fakeDiameterNode) Peers() []diameternode.PeerStatus { return f.peers }

func newDiameterTestDB(t *testing.T) *db.Database {
	t.Helper()

	testdb, err := db.NewDatabaseWithoutRaft(context.Background(), filepath.Join(t.TempDir(), "ella.db"))
	if err != nil {
		t.Fatalf("new database: %v", err)
	}

	t.Cleanup(func() { _ = testdb.Close() })

	return testdb
}

func getDiameter(t *testing.T, testdb *db.Database, cfg config.Config, node DiameterNode) DiameterStatus {
	t.Helper()

	rec := httptest.NewRecorder()
	GetDiameterStatus(testdb, cfg, node).ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/integrations/diameter", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Result DiameterStatus `json:"result"`
	}

	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	return resp.Result
}

func TestGetDiameterStatus(t *testing.T) {
	ctx := context.Background()
	testdb := newDiameterTestDB(t)

	smsc := db.SMSC{ID: "smsc-id", Name: "main", ServiceCentres: []string{"15550000000"}, Nodes: []db.SMSCNode{{ID: "node-a", DiameterIdentity: "smsc.example.org", Address: "2001:db8::10", Port: 3868}}}
	if err := testdb.CreateSMSC(ctx, &smsc); err != nil {
		t.Fatalf("create SMSC: %v", err)
	}

	if err := testdb.CreateIMSPeer(ctx, &db.IMSPeer{ID: "ims-id", Name: "cscf", Realm: "ims.example.net", Addresses: []string{"192.0.2.16/28"}}); err != nil {
		t.Fatalf("create IMS peer: %v", err)
	}

	if err := testdb.UpdateIMSConfiguration(ctx, &db.IMSSettings{HomeDomain: "ims.example.com"}, nil); err != nil {
		t.Fatalf("set the IMS home domain: %v", err)
	}

	since := time.Date(2026, 9, 29, 12, 0, 0, 0, time.FixedZone("EDT", -4*3600))

	cfg := config.Config{Interfaces: config.Interfaces{Diameter: config.DiameterInterface{Enabled: true, Address: "10.0.0.2", Port: 3868}}}

	got := getDiameter(t, testdb, cfg, fakeDiameterNode{peers: []diameternode.PeerStatus{{
		ID:      "smsc-node-a",
		Role:    "smsc",
		Host:    "smsc.example.org",
		Realm:   "example.org",
		Address: "2001:db8::10",
		Port:    3868,
		State:   diameter.PeerOpen,
		Since:   since,
	}, {
		ID:      "scscf.ims.example.net",
		Role:    "ims",
		Host:    "scscf.ims.example.net",
		Realm:   "ims.example.net",
		Address: "192.0.2.20",
		Inbound: true,
		State:   diameter.PeerOpen,
		Since:   since,
	}, {
		ID:      "intruder.example.org",
		Role:    "unknown",
		Host:    "intruder.example.org",
		Realm:   "example.org",
		Address: "192.0.2.30",
		Inbound: true,
		State:   diameter.PeerOpen,
		Since:   since,
	}, {
		ID:      "elsewhere.ims.example.net",
		Role:    "unknown",
		Host:    "elsewhere.ims.example.net",
		Realm:   "ims.example.net",
		Address: "198.51.100.1",
		Inbound: true,
		State:   diameter.PeerOpen,
		Since:   since,
	}}})

	if got.Realm == "" || len(got.ServedRealms) != 3 || got.ServedRealms[0] != got.Realm || got.ServedRealms[2] != "ims.example.com" {
		t.Fatalf("realms = %q %v", got.Realm, got.ServedRealms)
	}

	if !got.Listener.Enabled || !reflect.DeepEqual(got.Listener.Addresses, []string{"10.0.0.2"}) || got.Listener.Port != 3868 {
		t.Fatalf("listener = %+v", got.Listener)
	}

	wantPeers := []DiameterPeer{{
		ID:         "smsc-node-a",
		Role:       "smsc",
		Owner:      &DiameterPeerOwner{Kind: DiameterPeerOwnerSMSC, Name: "main"},
		OriginHost: "smsc.example.org",
		Realm:      "example.org",
		Address:    "2001:db8::10",
		Port:       3868,
		Direction:  "outbound",
		State:      "open",
		Since:      "2026-09-29T16:00:00Z",
	}, {
		ID:         "scscf.ims.example.net",
		Role:       "ims",
		Owner:      &DiameterPeerOwner{Kind: DiameterPeerOwnerIMSPeer, Name: "cscf"},
		OriginHost: "scscf.ims.example.net",
		Realm:      "ims.example.net",
		Address:    "192.0.2.20",
		Direction:  "inbound",
		State:      "open",
		Since:      "2026-09-29T16:00:00Z",
	}, {
		ID:         "intruder.example.org",
		Role:       "unknown",
		OriginHost: "intruder.example.org",
		Realm:      "example.org",
		Address:    "192.0.2.30",
		Direction:  "inbound",
		State:      "open",
		Since:      "2026-09-29T16:00:00Z",
	}, {
		ID:         "elsewhere.ims.example.net",
		Role:       "unknown",
		OriginHost: "elsewhere.ims.example.net",
		Realm:      "ims.example.net",
		Address:    "198.51.100.1",
		Direction:  "inbound",
		State:      "open",
		Since:      "2026-09-29T16:00:00Z",
	}}

	if !reflect.DeepEqual(got.Peers, wantPeers) {
		t.Fatalf("peers = %+v, want %+v", got.Peers, wantPeers)
	}
}

func TestGetDiameterStatusWithoutNode(t *testing.T) {
	got := getDiameter(t, newDiameterTestDB(t), config.Config{}, nil)

	if got.Listener.Enabled || got.Peers == nil || len(got.Peers) != 0 {
		t.Fatalf("diameter without a node = %+v", got)
	}
}

func TestGetDiameterStatusWithAnUnreadableInterface(t *testing.T) {
	previous := config.GetInterfaceIPsFunc
	config.GetInterfaceIPsFunc = func(string) ([]string, error) { return nil, errors.New("no such interface") }

	t.Cleanup(func() { config.GetInterfaceIPsFunc = previous })

	cfg := config.Config{Interfaces: config.Interfaces{Diameter: config.DiameterInterface{Enabled: true, Name: "missing0", Port: 3868}}}
	got := getDiameter(t, newDiameterTestDB(t), cfg, nil)

	if !got.Listener.Enabled || got.Listener.Addresses == nil || len(got.Listener.Addresses) != 0 {
		t.Fatalf("listener = %+v, want enabled with no addresses", got.Listener)
	}
}
