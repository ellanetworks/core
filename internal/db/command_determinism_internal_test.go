// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	ellaraft "github.com/ellanetworks/core/internal/raft"
	"github.com/ellanetworks/core/internal/sqn"
	"github.com/ellanetworks/core/internal/util/milenage"
	"github.com/google/uuid"
)

const (
	determinismIMSI        = "001010000000001"
	determinismStaticIMSI  = "001010000000002"
	determinismVisitorIMSI = "208930000000001"
	determinismPartnerID   = "partner"
	determinismPeerID      = "peer"
)

func newInitializedCommandTestDB(t *testing.T) *Database {
	t.Helper()

	path := filepath.Join(t.TempDir(), "db.sqlite3")

	database, err := NewDatabaseWithoutRaft(t.Context(), path)
	if err != nil {
		t.Fatalf("NewDatabaseWithoutRaft: %v", err)
	}

	t.Cleanup(func() { _ = database.Close() })

	if err := database.Initialize(t.Context()); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	return database
}

func copyDatabase(t *testing.T, database *Database) *Database {
	t.Helper()

	path := filepath.Join(t.TempDir(), "copy.sqlite3")

	if _, err := database.writer().ExecContext(t.Context(), "VACUUM INTO ?", path); err != nil {
		t.Fatalf("copy database: %v", err)
	}

	copied, err := NewDatabaseWithoutRaft(t.Context(), path)
	if err != nil {
		t.Fatalf("open copy: %v", err)
	}

	t.Cleanup(func() { _ = copied.Close() })

	return copied
}

func reversePhysicalOrder(t *testing.T, database *Database, table string) {
	t.Helper()

	conn, err := database.writer().Conn(t.Context())
	if err != nil {
		t.Fatalf("conn: %v", err)
	}

	defer func() { _ = conn.Close() }()

	for _, stmt := range []string{
		"PRAGMA foreign_keys = OFF",
		"CREATE TEMP TABLE reordered AS SELECT * FROM " + table + " ORDER BY rowid DESC",
		"DELETE FROM " + table,
		"INSERT INTO " + table + " SELECT * FROM reordered",
		"DROP TABLE reordered",
	} {
		if _, err := conn.ExecContext(t.Context(), stmt); err != nil {
			t.Fatalf("reorder %s: %s: %v", table, stmt, err)
		}
	}
}

func replicatedState(t *testing.T, database *Database) map[string][]string {
	t.Helper()

	state := map[string][]string{}

	for _, table := range replicatedTables {
		var n int
		if err := database.writer().QueryRowContext(t.Context(), "SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?", table).Scan(&n); err != nil {
			t.Fatalf("look up %s: %v", table, err)
		}

		if n == 0 {
			continue
		}

		rows, err := database.writer().QueryContext(t.Context(), "SELECT * FROM "+table)
		if err != nil {
			t.Fatalf("read %s: %v", table, err)
		}

		cols, err := rows.Columns()
		if err != nil {
			t.Fatalf("columns of %s: %v", table, err)
		}

		var out []string

		for rows.Next() {
			values := make([]any, len(cols))
			ptrs := make([]any, len(cols))

			for i := range values {
				ptrs[i] = &values[i]
			}

			if err := rows.Scan(ptrs...); err != nil {
				t.Fatalf("scan %s: %v", table, err)
			}

			out = append(out, fmt.Sprintf("%q", values))
		}

		if err := rows.Err(); err != nil {
			t.Fatalf("read %s: %v", table, err)
		}

		_ = rows.Close()

		sort.Strings(out)
		state[table] = out
	}

	return state
}

func applyOne(t *testing.T, database *Database, index uint64, leaderTime int64, cmds ...wireCommand) []commandResult {
	t.Helper()

	out, err := database.ApplyEntries(t.Context(), []ellaraft.Entry{entry(t, index, leaderTime, cmds...)})
	if err != nil {
		t.Fatalf("ApplyEntries: %v", err)
	}

	return resultsOf(t, out, 0)
}

func describeResults(results []commandResult) []string {
	out := make([]string, len(results))

	for i, r := range results {
		value, _ := json.Marshal(r.value)
		out[i] = fmt.Sprintf("%s err=%v", value, r.err)
	}

	return out
}

func newUUID(t *testing.T) string {
	t.Helper()

	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("uuid: %v", err)
	}

	return id.String()
}

type determinismFixture struct {
	dataNetworkID string
	userID        string
	sessionIDs    []string
	policy        *Policy
	leasedAddrs   []netip.Addr
}

func seedDeterminismFixture(t *testing.T, database *Database) determinismFixture {
	t.Helper()

	ctx := t.Context()

	createTestSubscriber(t, database, determinismIMSI)
	createTestSubscriber(t, database, determinismStaticIMSI)

	dn, err := database.GetDataNetwork(ctx, InitialDataNetworkName)
	if err != nil {
		t.Fatalf("GetDataNetwork: %v", err)
	}

	var addrs []netip.Addr

	for session := 1; session <= 5; session++ {
		addr, err := database.AllocateIPLease(ctx, dn.ID, "ipv4", determinismIMSI, session, "1")
		if err != nil {
			t.Fatalf("AllocateIPLease(%d): %v", session, err)
		}

		addrs = append(addrs, addr)
	}

	if _, err := database.ReleaseIPLease(ctx, dn.ID, "ipv4", determinismIMSI, 2, "1"); err != nil {
		t.Fatalf("ReleaseIPLease: %v", err)
	}

	userID, err := database.CreateUser(ctx, &User{Email: "admin@example.com", RoleID: RoleAdmin, HashedPassword: "hash"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	var sessionIDs []string

	for i := range 3 {
		s := &Session{ID: newUUID(t), UserID: userID, TokenHash: []byte{byte(i)}, CreatedAt: 1000, ExpiresAt: time.Now().Add(time.Hour).Unix()}
		if err := database.CreateSession(ctx, s); err != nil {
			t.Fatalf("CreateSession: %v", err)
		}

		sessionIDs = append(sessionIDs, s.ID)
	}

	policy, err := database.GetPolicy(ctx, InitialPolicyName)
	if err != nil {
		t.Fatalf("GetPolicy: %v", err)
	}

	for _, tok := range []string{"expiring", "valid"} {
		expires := time.Now().Add(time.Hour).Unix()
		if tok == "valid" {
			expires = time.Now().Add(100 * 365 * 24 * time.Hour).Unix()
		}

		if err := database.MintJoinTokenRecord(ctx, &ClusterJoinToken{ID: tok, ClaimsJSON: "{}", ExpiresAt: expires}); err != nil {
			t.Fatalf("MintJoinTokenRecord: %v", err)
		}
	}

	if err := database.CreateRoamingDiameterPeer(ctx, &RoamingDiameterPeer{ID: determinismPeerID, DiameterIdentity: "dea.example.org", Address: "10.0.0.7", Port: 3868}); err != nil {
		t.Fatalf("CreateRoamingDiameterPeer: %v", err)
	}

	profile, err := database.GetProfile(ctx, InitialProfileName)
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}

	peerID := determinismPeerID
	if err := database.CreateRoamingPartner(ctx, &RoamingPartner{ID: determinismPartnerID, Name: "partner", PLMNs: []RoamingPLMN{{Mcc: "208", Mnc: "93"}}, ProfileID: profile.ID, DiameterPeerID: &peerID}); err != nil {
		t.Fatalf("CreateRoamingPartner: %v", err)
	}

	if err := database.SetRoamingPartnerWithdrawing(ctx, determinismPartnerID, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("SetRoamingPartnerWithdrawing: %v", err)
	}

	return determinismFixture{dataNetworkID: dn.ID, userID: userID, sessionIDs: sessionIDs, policy: policy, leasedAddrs: addrs}
}

func TestCommandsApplyIdenticallyOnEveryNode(t *testing.T) {
	leader := newInitializedCommandTestDB(t)
	f := seedDeterminismFixture(t, leader)

	follower := copyDatabase(t, leader)
	reversePhysicalOrder(t, follower, IPLeasesTableName)
	reversePhysicalOrder(t, follower, SessionsTableName)

	if got, want := replicatedState(t, follower), replicatedState(t, leader); !reflect.DeepEqual(got, want) {
		t.Fatal("the copied database does not start from the same replicated state")
	}

	leaderTime := time.Now().Add(10 * 365 * 24 * time.Hour).UnixNano()
	leaderSeconds := leaderTime / int64(time.Second)
	index := nextIndex(t, leader)

	staticAddr := netip.MustParseAddr("10.45.3.200").As16()
	remote := "10.10.0.0/16"
	ruleID := newUUID(t)
	staticID := newUUID(t)
	allocatedID := newUUID(t)
	routeID := newUUID(t)

	cmds := []wireCommand{
		command(t, "AllocateIPLease", &allocateIPLeasePayload{PoolID: f.dataNetworkID, PoolType: "ipv4", IMSI: determinismIMSI, SessionID: 6, NodeID: "1", LeaseID: allocatedID}),
		command(t, "CreateStaticLease", &IPLease{ID: staticID, PoolID: f.dataNetworkID, PoolType: "ipv4", AddressBin: staticAddr[:], IMSI: determinismStaticIMSI}),
		command(t, "ReplaceFramedRoutes", &framedRoutesPayload{Imsi: determinismIMSI, DataNetworkID: f.dataNetworkID, Prefixes: []string{"10.200.0.0/24"}, IDs: []string{routeID}}),
		command(t, "UpdatePolicyWithRules", &policyWithRulesPayload{Policy: *f.policy, Rules: &PolicyRulesInput{Uplink: []PolicyRuleInput{{Description: "r", RemotePrefix: &remote, Action: "allow"}}}, RuleIDs: []string{ruleID}}),
		command(t, "RegisterUE", &registerUEPayload{Imsi: determinismIMSI, Type: UERegistrationTypeMME, NodeID: "1"}),
		command(t, "PurgeUERegistration", &purgeUERegistrationPayload{Imsi: determinismIMSI, Type: UERegistrationTypeMME, NodeID: "1"}),
		command(t, "DeleteOldestSessions", &DeleteOldestArgs{UserID: f.userID, Limit: 1}),
		command(t, "RedeemJoinToken", &redeemJoinTokenPayload{TokenID: "expiring", NodeID: "2", Fingerprint: "sha256:aa", CertPEM: "PEM-2"}),
		command(t, "RedeemJoinToken", &redeemJoinTokenPayload{TokenID: "valid", NodeID: "3", Fingerprint: "sha256:bb", CertPEM: "PEM-3"}),
		command(t, "RegisterVisitorUE", &registerVisitorUEPayload{Imsi: determinismVisitorIMSI, PartnerID: determinismPartnerID, Type: UERegistrationTypeMME, NodeID: "1"}),
	}

	leaderResults := applyOne(t, leader, index, leaderTime, cmds...)

	time.Sleep(1100 * time.Millisecond)

	followerResults := applyOne(t, follower, index, leaderTime, cmds...)

	if got, want := describeResults(followerResults), describeResults(leaderResults); !reflect.DeepEqual(got, want) {
		t.Fatalf("results differ:\nleader   %v\nfollower %v", want, got)
	}

	if got, want := replicatedState(t, follower), replicatedState(t, leader); !reflect.DeepEqual(got, want) {
		for table := range want {
			if !reflect.DeepEqual(got[table], want[table]) {
				t.Errorf("%s differs:\nleader   %v\nfollower %v", table, want[table], got[table])
			}
		}

		t.FailNow()
	}

	for i, r := range leaderResults {
		if i == 7 {
			if !errors.Is(r.err, ErrJoinTokenExpired) {
				t.Fatalf("redeeming a token that expires before the leader time: err = %v, want ErrJoinTokenExpired", r.err)
			}

			continue
		}

		if r.err != nil {
			t.Fatalf("command %d (%s): %v", i, cmds[i].Op, r.err)
		}
	}

	want := f.leasedAddrs[4].Next().String()
	if leaderResults[0].value != want {
		t.Fatalf("allocated %v, want %s: the next free address follows the latest lease, not the physical row order", leaderResults[0].value, want)
	}

	if version, _ := leaderResults[4].value.(int64); version != int64(index) {
		t.Fatalf("registration version = %d, want the entry index %d", version, index)
	}

	ctx := t.Context()

	lease, err := leader.GetLeaseBySession(ctx, f.dataNetworkID, "ipv4", 6, determinismIMSI)
	if err != nil {
		t.Fatalf("GetLeaseBySession: %v", err)
	}

	if lease.ID != allocatedID || lease.CreatedAt != leaderSeconds {
		t.Fatalf("lease = %s created %d, want %s created %d", lease.ID, lease.CreatedAt, allocatedID, leaderSeconds)
	}

	static, err := leader.GetStaticLease(ctx, f.dataNetworkID, "ipv4", determinismStaticIMSI)
	if err != nil {
		t.Fatalf("GetStaticLease: %v", err)
	}

	if static.ID != staticID || static.CreatedAt != leaderSeconds {
		t.Fatalf("static lease = %s created %d, want %s created %d", static.ID, static.CreatedAt, staticID, leaderSeconds)
	}

	rule, err := leader.GetNetworkRule(ctx, ruleID)
	if err != nil {
		t.Fatalf("GetNetworkRule: %v", err)
	}

	if rule.CreatedAt.UnixNano() != leaderTime {
		t.Fatalf("network rule created at %v, want the leader time %v", rule.CreatedAt, time.Unix(0, leaderTime))
	}

	visitor, err := leader.GetVisitor(ctx, determinismVisitorIMSI)
	if err != nil {
		t.Fatalf("GetVisitor: %v", err)
	}

	if visitor.Since != leaderSeconds {
		t.Fatalf("visitor since %d, want %d", visitor.Since, leaderSeconds)
	}

	token, err := leader.GetJoinToken(ctx, "valid")
	if err != nil {
		t.Fatalf("GetJoinToken: %v", err)
	}

	if token.ConsumedAt != leaderSeconds {
		t.Fatalf("token consumed at %d, want %d", token.ConsumedAt, leaderSeconds)
	}

	var remaining []string

	rows, err := leader.writer().QueryContext(ctx, "SELECT id FROM sessions ORDER BY id")
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}

	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan: %v", err)
		}

		remaining = append(remaining, id)
	}

	_ = rows.Close()

	sorted := append([]string(nil), f.sessionIDs...)
	sort.Strings(sorted)

	if !reflect.DeepEqual(remaining, sorted[1:]) {
		t.Fatalf("remaining sessions = %v, want all but the smallest id of %v", remaining, sorted)
	}
}

func TestCommandsRequireCallerGeneratedIDs(t *testing.T) {
	database := newInitializedCommandTestDB(t)
	createTestSubscriber(t, database, determinismIMSI)

	dn, err := database.GetDataNetwork(t.Context(), InitialDataNetworkName)
	if err != nil {
		t.Fatalf("GetDataNetwork: %v", err)
	}

	policy, err := database.GetPolicy(t.Context(), InitialPolicyName)
	if err != nil {
		t.Fatalf("GetPolicy: %v", err)
	}

	addr := netip.MustParseAddr("10.45.1.1").As16()

	cmds := []wireCommand{
		command(t, "AllocateIPLease", &allocateIPLeasePayload{PoolID: dn.ID, PoolType: "ipv4", IMSI: determinismIMSI, SessionID: 1, NodeID: "1"}),
		command(t, "CreateLease", &IPLease{PoolID: dn.ID, PoolType: "ipv4", AddressBin: addr[:], IMSI: determinismIMSI, Type: "dynamic"}),
		command(t, "CreateStaticLease", &IPLease{PoolID: dn.ID, PoolType: "ipv4", AddressBin: addr[:], IMSI: determinismIMSI}),
		command(t, "ReplaceFramedRoutes", &framedRoutesPayload{Imsi: determinismIMSI, DataNetworkID: dn.ID, Prefixes: []string{"10.200.0.0/24"}}),
		command(t, "UpdatePolicyWithRules", &policyWithRulesPayload{Policy: *policy, Rules: &PolicyRulesInput{Uplink: []PolicyRuleInput{{Action: "allow"}}}}),
	}

	for i, r := range applyOne(t, database, nextIndex(t, database), time.Now().UnixNano(), cmds...) {
		if !errors.Is(r.err, ErrInvalidCommand) {
			t.Errorf("%s without an id: err = %v, want ErrInvalidCommand", cmds[i].Op, r.err)
		}
	}
}

func subscriberCountMatches(t *testing.T, database *Database) int {
	t.Helper()

	counted, err := database.subscriberCount(t.Context(), database.writer())
	if err != nil {
		t.Fatalf("subscriberCount: %v", err)
	}

	actual, err := queryCount(t.Context(), database.writer(), countSubscribersAndVisitorsSQL)
	if err != nil {
		t.Fatalf("count rows: %v", err)
	}

	if counted != actual {
		t.Fatalf("subscriber_count = %d, subscribers and visitors = %d", counted, actual)
	}

	return counted
}

func TestSubscriberCountFollowsSubscribersAndVisitors(t *testing.T) {
	database := newInitializedCommandTestDB(t)
	ctx := t.Context()

	base := subscriberCountMatches(t, database)

	for i := range 6 {
		createTestSubscriber(t, database, fmt.Sprintf("00101000000010%d", i))
	}

	for _, i := range []int{1, 4} {
		if err := database.DeleteSubscriber(ctx, fmt.Sprintf("00101000000010%d", i)); err != nil {
			t.Fatalf("DeleteSubscriber: %v", err)
		}
	}

	if err := database.DeleteSubscriber(ctx, "001010000000199"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleting a missing subscriber: err = %v", err)
	}

	if err := database.CreateRoamingDiameterPeer(ctx, &RoamingDiameterPeer{ID: determinismPeerID, DiameterIdentity: "dea.example.org", Address: "10.0.0.7", Port: 3868}); err != nil {
		t.Fatalf("CreateRoamingDiameterPeer: %v", err)
	}

	profile, err := database.GetProfile(ctx, InitialProfileName)
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}

	peerID := determinismPeerID
	if err := database.CreateRoamingPartner(ctx, &RoamingPartner{ID: determinismPartnerID, Name: "partner", PLMNs: []RoamingPLMN{{Mcc: "208", Mnc: "93"}}, ProfileID: profile.ID, DiameterPeerID: &peerID}); err != nil {
		t.Fatalf("CreateRoamingPartner: %v", err)
	}

	member := newUUID(t)

	for i := range 4 {
		node := "1"
		if i >= 2 {
			node = member
		}

		if _, err := database.RegisterVisitorUE(ctx, fmt.Sprintf("20893000000000%d", i), determinismPartnerID, UERegistrationTypeMME, node, ""); err != nil {
			t.Fatalf("RegisterVisitorUE: %v", err)
		}
	}

	if _, err := database.RegisterVisitorUE(ctx, "208930000000000", determinismPartnerID, UERegistrationTypeMME, "1", ""); err != nil {
		t.Fatalf("re-registering a visitor: %v", err)
	}

	if err := database.CreateSubscriber(ctx, &Subscriber{Imsi: "208930000000000", PermanentKey: "465b5ce8b199b49faa5f0a2ee238a6bc", Opc: "cd63cb71954a9f4e48a5994e37a02baf", SequenceNumber: "000000000000", ProfileID: profile.ID}); !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("creating a subscriber with a visitor's IMSI: err = %v", err)
	}

	subscriberCountMatches(t, database)

	if err := database.PurgeUERegistration(ctx, "208930000000000", UERegistrationTypeMME, "1"); err != nil {
		t.Fatalf("PurgeUERegistration: %v", err)
	}

	if err := database.UpsertClusterMember(ctx, &ClusterMember{NodeID: member, APIAddress: "10.0.0.9:5002", BinaryVersion: "test"}); err != nil {
		t.Fatalf("UpsertClusterMember: %v", err)
	}

	if err := database.DeleteClusterMember(ctx, member); err != nil {
		t.Fatalf("DeleteClusterMember: %v", err)
	}

	if got := subscriberCountMatches(t, database); got != base+4+1 {
		t.Fatalf("subscriber_count = %d, want %d", got, base+5)
	}

	database.maxSubscribers = base + 5

	err = database.CreateSubscriber(ctx, &Subscriber{Imsi: "001010000000300", PermanentKey: "465b5ce8b199b49faa5f0a2ee238a6bc", Opc: "cd63cb71954a9f4e48a5994e37a02baf", SequenceNumber: "000000000000", ProfileID: profile.ID})
	if !errors.Is(err, ErrSubscriberLimit) {
		t.Fatalf("creating a subscriber at the limit: err = %v, want ErrSubscriberLimit", err)
	}

	if _, err := database.RegisterVisitorUE(ctx, "208930000000009", determinismPartnerID, UERegistrationTypeMME, "1", ""); !errors.Is(err, ErrSubscriberLimit) {
		t.Fatalf("registering a visitor at the limit: err = %v, want ErrSubscriberLimit", err)
	}

	subscriberCountMatches(t, database)
}

func TestMigrationV31SeedsTheSubscriberCount(t *testing.T) {
	database := newInitializedCommandTestDB(t)

	for i := range 3 {
		createTestSubscriber(t, database, fmt.Sprintf("00101000000020%d", i))
	}

	ctx := t.Context()

	for _, stmt := range []string{"DROP TABLE " + SubscriberCountTableName, "DROP INDEX idx_leases_pool_latest"} {
		if _, err := database.writer().ExecContext(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}

	tx, err := database.writer().BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}

	if err := migrateV31(ctx, tx); err != nil {
		_ = tx.Rollback()

		t.Fatalf("migrateV31: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	if got := subscriberCountMatches(t, database); got < 3 {
		t.Fatalf("subscriber_count = %d, want at least 3", got)
	}
}

func generateAUTS(t *testing.T, rand []byte, sqnMS string) string {
	t.Helper()

	k, _ := hex.DecodeString("465b5ce8b199b49faa5f0a2ee238a6bc")
	opc, _ := hex.DecodeString("cd63cb71954a9f4e48a5994e37a02baf")
	ms, _ := hex.DecodeString(sqnMS)

	auts, err := milenage.GenerateAUTS(opc, k, rand, ms)
	if err != nil {
		t.Fatalf("GenerateAUTS: %v", err)
	}

	return hex.EncodeToString(auts)
}

func storedSQN(t *testing.T, database *Database, imsi string) string {
	t.Helper()

	sub, err := database.GetSubscriber(t.Context(), imsi)
	if err != nil {
		t.Fatalf("GetSubscriber: %v", err)
	}

	return sub.SequenceNumber
}

func TestSQNResyncIsVerifiedBeforeProposing(t *testing.T) {
	const sqnMS = "00000000105e"

	rand := make([]byte, 16)
	randHex := hex.EncodeToString(rand)

	for _, domain := range []string{"", SQNDomainIMS} {
		t.Run("domain "+domain, func(t *testing.T) {
			database := newInitializedCommandTestDB(t)
			createTestSubscriber(t, database, determinismIMSI)

			want, err := sqn.NextFromResync(sqnMS)
			if domain == SQNDomainIMS {
				want, err = sqn.NextIMSFromResync("000000000000", sqnMS)
			}

			if err != nil {
				t.Fatalf("expected SQN: %v", err)
			}

			auts := generateAUTS(t, rand, sqnMS)

			payload := &AdvanceSQNPayload{IMSI: determinismIMSI, ResyncAuts: auts, ResyncRand: randHex, Domain: domain}
			if err := database.verifySQNResync(t.Context(), payload); err != nil {
				t.Fatalf("verifySQNResync: %v", err)
			}

			if payload.ResyncSQNms != sqnMS || payload.ResyncAuts != "" || payload.ResyncRand != "" {
				t.Fatalf("payload = %+v, want only the SQN_MS %s", payload, sqnMS)
			}

			creds, err := database.advanceSubscriberSQN(t.Context(), &AdvanceSQNPayload{IMSI: determinismIMSI, ResyncAuts: auts, ResyncRand: randHex, Domain: domain})
			if err != nil {
				t.Fatalf("advance with a resync: %v", err)
			}

			if creds.SequenceNumber != want || storedSQN(t, database, determinismIMSI) != want {
				t.Fatalf("sequence number = %s, want %s", creds.SequenceNumber, want)
			}

			before := fsmState(t, database).lastApplied
			bad := []byte(auts)
			bad[len(bad)-1] ^= 0x01

			_, err = database.advanceSubscriberSQN(t.Context(), &AdvanceSQNPayload{IMSI: determinismIMSI, ResyncAuts: string(bad), ResyncRand: randHex, Domain: domain})
			if !errors.Is(err, ErrInvalidCommand) {
				t.Fatalf("bad AUTS: err = %v, want ErrInvalidCommand", err)
			}

			if after := fsmState(t, database).lastApplied; after != before {
				t.Fatalf("a bad AUTS reached the log: lastApplied %d -> %d", before, after)
			}

			if got := storedSQN(t, database, determinismIMSI); got != want {
				t.Fatalf("stored sequence number = %s after a bad AUTS, want %s", got, want)
			}
		})
	}
}

func TestSQNCommandAcceptsTheOldPayloadShape(t *testing.T) {
	const sqnMS = "00000000105e"

	rand := make([]byte, 16)
	randHex := hex.EncodeToString(rand)

	for _, domain := range []string{"", SQNDomainIMS} {
		t.Run("domain "+domain, func(t *testing.T) {
			oldShape := newInitializedCommandTestDB(t)
			createTestSubscriber(t, oldShape, determinismIMSI)

			newShape := copyDatabase(t, oldShape)
			index := nextIndex(t, oldShape)
			auts := generateAUTS(t, rand, sqnMS)

			oldResult := applyOne(t, oldShape, index, time.Now().UnixNano(), command(t, "AdvanceSubscriberSQN", &AdvanceSQNPayload{IMSI: determinismIMSI, ResyncAuts: auts, ResyncRand: randHex, Domain: domain}))
			newResult := applyOne(t, newShape, index, time.Now().UnixNano(), command(t, "AdvanceSubscriberSQN", &AdvanceSQNPayload{IMSI: determinismIMSI, ResyncSQNms: sqnMS, Domain: domain}))

			if oldResult[0].err != nil || newResult[0].err != nil {
				t.Fatalf("old shape err = %v, new shape err = %v", oldResult[0].err, newResult[0].err)
			}

			if !reflect.DeepEqual(oldResult[0].value, newResult[0].value) {
				t.Fatalf("old shape = %+v, new shape = %+v", oldResult[0].value, newResult[0].value)
			}

			bad := []byte(auts)
			bad[len(bad)-1] ^= 0x01

			rejected := applyOne(t, oldShape, nextIndex(t, oldShape), time.Now().UnixNano(), command(t, "AdvanceSubscriberSQN", &AdvanceSQNPayload{IMSI: determinismIMSI, ResyncAuts: string(bad), ResyncRand: randHex, Domain: domain}))
			if !errors.Is(rejected[0].err, ErrInvalidCommand) {
				t.Fatalf("old shape with a bad AUTS: err = %v, want ErrInvalidCommand", rejected[0].err)
			}
		})
	}
}

func TestDeleteClusterMemberRejectsAMalformedNodeIDBeforeProposing(t *testing.T) {
	database := newCommandTestDB(t)
	before := fsmState(t, database).lastApplied

	err := database.DeleteClusterMember(t.Context(), "node-1")
	if !errors.Is(err, ErrInvalidCommand) {
		t.Fatalf("err = %v, want ErrInvalidCommand", err)
	}

	if after := fsmState(t, database).lastApplied; after != before {
		t.Fatalf("a malformed node ID reached the log: lastApplied %d -> %d", before, after)
	}
}

func TestApplyEntriesHaltsOnAPanickingCommand(t *testing.T) {
	database := newCommandTestDB(t)

	registerTestCommand(t, "TestWriteThenPanic", 1, func(db *Database, ctx context.Context, p *json.RawMessage) (any, error) {
		if _, err := db.applyCreateProfile(ctx, newTestProfile(t, "panicked")); err != nil {
			return nil, err
		}

		panic("boom")
	})

	index := nextIndex(t, database)

	_, err := database.ApplyEntries(t.Context(), []ellaraft.Entry{entry(t, index, time.Now().UnixNano(),
		command(t, "CreateProfile", newTestProfile(t, "before")),
		command(t, "TestWriteThenPanic", struct{}{}),
	)})

	var fatal *fatalApplyError
	if !errors.As(err, &fatal) || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err = %v, want a fatal apply error naming the panic", err)
	}

	if profileExists(t, database, "before") || profileExists(t, database, "panicked") {
		t.Fatal("the entry that panicked was partly committed")
	}

	if got := fsmState(t, database).lastApplied; got != index-1 {
		t.Fatalf("lastApplied = %d, want %d", got, index-1)
	}
}
