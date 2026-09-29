// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"reflect"
	"testing"
	"time"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/internal/smsf"
)

type fakeDiameterNode struct {
	identity    smsf.Identity
	identityErr error
	peers       []smsf.PeerStatus
}

func (f fakeDiameterNode) Identity(context.Context) (smsf.Identity, error) {
	return f.identity, f.identityErr
}

func (f fakeDiameterNode) Peers() []smsf.PeerStatus { return f.peers }

func getDiameter(t *testing.T, node DiameterNode) DiameterStatus {
	t.Helper()

	rec := httptest.NewRecorder()
	GetDiameterStatus(node).ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/networking/diameter", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
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
	since := time.Date(2026, 9, 29, 12, 0, 0, 0, time.FixedZone("EDT", -4*3600))

	got := getDiameter(t, fakeDiameterNode{
		identity: smsf.Identity{Host: "mmec01.mmegi8204.mme.epc.mnc001.mcc001.3gppnetwork.org", Realm: "epc.mnc001.mcc001.3gppnetwork.org"},
		peers: []smsf.PeerStatus{{
			Role:    smsf.PeerRoleSMSC,
			Host:    "smsc.example.org",
			Realm:   "example.org",
			Address: netip.MustParseAddrPort("[2001:db8::10]:3868"),
			State:   diameter.PeerOpen,
			Since:   since,
		}},
	})

	want := DiameterStatus{
		Host:  "mmec01.mmegi8204.mme.epc.mnc001.mcc001.3gppnetwork.org",
		Realm: "epc.mnc001.mcc001.3gppnetwork.org",
		Peers: []DiameterPeer{{
			Role:    "smsc",
			Host:    "smsc.example.org",
			Realm:   "example.org",
			Address: "2001:db8::10",
			Port:    3868,
			State:   "open",
			Since:   "2026-09-29T16:00:00Z",
		}},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("diameter = %+v, want %+v", got, want)
	}
}

func TestGetDiameterStatusWithoutIdentityOrPeers(t *testing.T) {
	got := getDiameter(t, fakeDiameterNode{identityErr: errors.New("no AMF Pointer yet")})

	if got.Host != "" || got.Realm != "" || got.Peers == nil || len(got.Peers) != 0 {
		t.Fatalf("diameter = %+v, want no identity and an empty peer list", got)
	}

	if got := getDiameter(t, nil); got.Peers == nil || len(got.Peers) != 0 {
		t.Fatalf("diameter without a node = %+v", got)
	}
}
