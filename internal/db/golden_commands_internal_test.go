// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	ellaraft "github.com/ellanetworks/core/internal/raft"
)

const (
	goldenLogPath   = "testdata/golden_commands/log.jsonl"
	goldenStatePath = "testdata/golden_commands/state.txt"
)

var goldenRejectedOperations = map[string]string{
	"CreateSMSCPeer": "SMSC peers became SMSC nodes in schema 26; a forwarded legacy write is rejected with ErrRetiredOperation",
	"UpdateSMSCPeer": "SMSC peers became SMSC nodes in schema 26; a forwarded legacy write is rejected with ErrRetiredOperation",
	"DeleteSMSCPeer": "SMSC peers became SMSC nodes in schema 26; a forwarded legacy write is rejected with ErrRetiredOperation",
}

type goldenEntry struct {
	Index uint64          `json:"index"`
	Term  uint64          `json:"term"`
	Entry json.RawMessage `json:"entry"`
}

type goldenRun struct {
	t   *testing.T
	ctx context.Context
	db  *Database
}

func (g *goldenRun) must(what string, err error) {
	g.t.Helper()

	if err != nil {
		g.t.Fatalf("%s: %v", what, err)
	}
}

func (g *goldenRun) raw(op string, payload any) error {
	g.t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		g.t.Fatalf("marshal %s: %v", op, err)
	}

	_, err = g.db.proposeCommand(g.ctx, op, body)

	return err
}

func runGoldenWorkload(t *testing.T, database *Database) {
	g := &goldenRun{t: t, ctx: t.Context(), db: database}
	ctx := g.ctx
	d := database

	g.must("Initialize", d.Initialize(ctx))
	g.must("SetJWTSecret", d.SetJWTSecret(ctx, bytes.Repeat([]byte{7}, 32)))
	g.must("RotateJWTSecret", d.RotateJWTSecret(ctx, bytes.Repeat([]byte{8}, 32)))
	g.must("UpdateRadioEventSettings", d.UpdateRadioEventSettings(ctx, false))
	g.must("SetRetentionPolicy", d.SetRetentionPolicy(ctx, &RetentionPolicy{Category: CategoryAuditLogs, Days: 9}))

	g.must("UpdateOperatorTracking", d.UpdateOperatorTracking(ctx, []string{"000001", "000002"}))
	g.must("UpdateOperatorCode", d.UpdateOperatorCode(ctx, "0123456789abcdef0123456789abcdef"))
	g.must("UpdateOperatorSecurityAlgorithms", d.UpdateOperatorSecurityAlgorithms(ctx, []string{"NEA2", "NEA1", "NEA0"}, []string{"NIA2", "NIA1"}))
	g.must("UpdateOperatorSPN", d.UpdateOperatorSPN(ctx, "Golden Network", "Golden"))
	g.must("UpdateOperatorAMFIdentity", d.UpdateOperatorAMFIdentity(ctx, 2, 3))
	g.must("UpdateOperatorClusterID", d.UpdateOperatorClusterID(ctx, "golden-cluster"))
	g.must("UpdateOperatorID", d.UpdateOperatorID(ctx, "001", "01"))

	g.must("CreateHomeNetworkKey", d.CreateHomeNetworkKey(ctx, &HomeNetworkKey{ID: "01890000-0000-7000-8000-00000000a001", KeyIdentifier: 5, Scheme: "A", PrivateKey: strings.Repeat("ab", 32)}))
	g.must("DeleteHomeNetworkKey", d.DeleteHomeNetworkKey(ctx, "01890000-0000-7000-8000-00000000a001"))

	dn := &DataNetwork{Name: "golden", IPv4Pool: "10.77.0.0/24", IPv6Pool: "2001:db8:77::/48", DNS: "8.8.8.8", MTU: 1400}
	g.must("CreateDataNetwork", d.CreateDataNetwork(ctx, dn))

	dn, err := d.GetDataNetwork(ctx, "golden")
	g.must("GetDataNetwork", err)

	dn.MTU = 1420
	g.must("UpdateDataNetwork", d.UpdateDataNetwork(ctx, dn))

	sd := "102030"
	slice := &NetworkSlice{Sst: 1, Sd: &sd, Name: "golden-slice"}
	g.must("CreateNetworkSlice", d.CreateNetworkSlice(ctx, slice))

	slice, err = d.GetNetworkSlice(ctx, "golden-slice")
	g.must("GetNetworkSlice", err)

	slice.Sst = 2
	g.must("UpdateNetworkSlice", d.UpdateNetworkSlice(ctx, slice))

	g.must("CreateSliceDataNetwork", d.CreateSliceDataNetwork(ctx, &SliceDataNetwork{SliceID: slice.ID, DataNetworkID: dn.ID, NAT: true}))
	g.must("UpdateSliceDataNetwork", d.UpdateSliceDataNetwork(ctx, &SliceDataNetwork{SliceID: slice.ID, DataNetworkID: dn.ID, LocalSwitch: true}))

	profile := &Profile{Name: "golden-profile", UeAmbrUplink: "1 Gbps", UeAmbrDownlink: "2 Gbps", Allow4G: true, Allow5G: true}
	g.must("CreateProfile", d.CreateProfile(ctx, profile))

	profile, err = d.GetProfile(ctx, "golden-profile")
	g.must("GetProfile", err)

	profile.UeAmbrUplink = "500 Mbps"
	g.must("UpdateProfile", d.UpdateProfile(ctx, profile))

	policy := &Policy{Name: "golden-policy", ProfileID: profile.ID, SliceID: slice.ID, DataNetworkID: dn.ID, Var5qi: 9, Arp: 1, SessionAmbrUplink: "100 Mbps", SessionAmbrDownlink: "200 Mbps"}
	g.must("CreatePolicy", d.CreatePolicy(ctx, policy))

	policy, err = d.GetPolicy(ctx, "golden-policy")
	g.must("GetPolicy", err)

	policy.Var5qi = 8
	g.must("UpdatePolicy", d.UpdatePolicy(ctx, policy))
	g.must("SetDefaultPolicy", d.SetDefaultPolicy(ctx, profile.ID, "golden-policy"))

	remote := "10.10.0.0/16"
	ruleID, err := d.CreateNetworkRule(ctx, &NetworkRule{PolicyID: policy.ID, Description: "web", Direction: "uplink", RemotePrefix: &remote, Protocol: 6, PortLow: 80, PortHigh: 443, Action: "allow", Precedence: 10})
	g.must("CreateNetworkRule", err)
	g.must("UpdateNetworkRule", d.UpdateNetworkRule(ctx, &NetworkRule{ID: ruleID, PolicyID: policy.ID, Description: "web2", Direction: "uplink", RemotePrefix: &remote, Protocol: 6, PortLow: 80, PortHigh: 80, Action: "deny", Precedence: 11}))
	g.must("DeleteNetworkRule", d.DeleteNetworkRule(ctx, ruleID))

	defaultProfile, err := d.GetProfile(ctx, InitialProfileName)
	g.must("GetProfile default", err)

	rulesPolicy := &Policy{Name: "golden-rules", ProfileID: defaultProfile.ID, SliceID: slice.ID, DataNetworkID: dn.ID, Var5qi: 7, Arp: 2, SessionAmbrUplink: "10 Mbps", SessionAmbrDownlink: "20 Mbps"}
	g.must("CreatePolicyWithRules", d.CreatePolicyWithRules(ctx, rulesPolicy, &PolicyRulesInput{
		Uplink:   []PolicyRuleInput{{Description: "u1", Protocol: 6, PortLow: 22, PortHigh: 22, Action: "allow"}, {Description: "u2", Protocol: 17, PortLow: 53, PortHigh: 53, Action: "allow"}},
		Downlink: []PolicyRuleInput{{Description: "d1", RemotePrefix: &remote, Action: "deny"}},
	}))

	rulesPolicy, err = d.GetPolicy(ctx, "golden-rules")
	g.must("GetPolicy rules", err)

	g.must("UpdatePolicyWithRules", d.UpdatePolicyWithRules(ctx, rulesPolicy, &PolicyRulesInput{Uplink: []PolicyRuleInput{{Description: "only", Protocol: 1, Action: "allow"}}}))
	g.must("DeleteNetworkRulesByPolicyID", d.DeleteNetworkRulesByPolicyID(ctx, rulesPolicy.ID))
	g.must("DeletePolicy", d.DeletePolicy(ctx, "golden-rules"))

	const (
		imsi        = "001010000000101"
		staticIMSI  = "001010000000102"
		ki          = "465b5ce8b199b49faa5f0a2ee238a6bc"
		opc         = "cd63cb71954a9f4e48a5994e37a02baf"
		visitorIMSI = "208930000000101"
	)

	for _, s := range []string{imsi, staticIMSI, "001010000000103"} {
		g.must("CreateSubscriber "+s, d.CreateSubscriber(ctx, &Subscriber{Imsi: s, PermanentKey: ki, Opc: opc, SequenceNumber: "000000000020", ProfileID: profile.ID}))
	}

	g.must("UpdateSubscriberProfile", d.UpdateSubscriberProfile(ctx, &Subscriber{Imsi: "001010000000103", ProfileID: profile.ID, Description: "updated"}))
	g.must("EditSubscriberSequenceNumber", d.EditSubscriberSequenceNumber(ctx, imsi, "000000000040"))

	_, err = d.AdvanceSubscriberSQN(ctx, imsi, "", "")
	g.must("AdvanceSubscriberSQN", err)

	rand := bytes.Repeat([]byte{0x5a}, 16)
	_, err = d.AdvanceSubscriberSQN(ctx, imsi, generateAUTS(t, rand, "000000000200"), hex.EncodeToString(rand))
	g.must("AdvanceSubscriberSQN resync", err)

	g.must("DeleteSubscriber", d.DeleteSubscriber(ctx, "001010000000103"))

	day := DaysSinceEpoch(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC))
	g.must("IncrementDailyUsage", d.IncrementDailyUsage(ctx, DailyUsage{EpochDay: day, IMSI: imsi, BytesUplink: 10, BytesDownlink: 20}))
	g.must("IncrementDailyUsageBatch", d.IncrementDailyUsageBatch(ctx, []DailyUsage{{EpochDay: day, IMSI: imsi, BytesUplink: 1, BytesDownlink: 2}, {EpochDay: day + 1, IMSI: staticIMSI, BytesUplink: 3, BytesDownlink: 4}}))
	g.must("DeleteOldDailyUsage", d.DeleteOldDailyUsage(ctx, 100000))
	g.must("ClearDailyUsage", d.ClearDailyUsage(ctx))

	addr, err := d.AllocateIPLease(ctx, dn.ID, "ipv4", imsi, 1, "1")
	g.must("AllocateIPLease", err)

	for session := 2; session <= 4; session++ {
		_, err = d.AllocateIPLease(ctx, dn.ID, "ipv4", imsi, session, "1")
		g.must("AllocateIPLease", err)
	}

	_, err = d.ReleaseIPLease(ctx, dn.ID, "ipv4", imsi, 2, "1")
	g.must("ReleaseIPLease", err)

	_, err = d.AllocateIPv6Lease(ctx, dn.ID, "ipv6", imsi, 1, "1")
	g.must("AllocateIPv6Lease", err)

	lease, err := d.GetLeaseBySession(ctx, dn.ID, "ipv4", 1, imsi)
	g.must("GetLeaseBySession", err)

	if lease.Address() != addr {
		t.Fatalf("lease address %s, allocated %s", lease.Address(), addr)
	}

	g.must("UpdateLeaseSession", d.UpdateLeaseSession(ctx, lease.ID, 9))
	g.must("UpdateLeaseNode", d.UpdateLeaseNode(ctx, lease.ID, "2", 9))
	g.must("DeleteDynamicLease", d.DeleteDynamicLease(ctx, lease.ID))

	session := 5
	g.must("CreateLease", d.CreateLease(ctx, &IPLease{ID: "01890000-0000-7000-8000-00000000b001", PoolID: dn.ID, PoolType: "ipv4", IMSI: imsi, SessionID: &session, Type: "dynamic", CreatedAt: 1767225600, NodeID: "2"}, netip.MustParseAddr("10.77.0.200")))
	g.must("DeleteDynamicLeasesByNode", d.DeleteDynamicLeasesByNode(ctx, "2"))
	g.must("DeleteAllDynamicLeases", d.DeleteAllDynamicLeases(ctx))

	g.must("CreateStaticLease", d.CreateStaticLease(ctx, staticIMSI, dn.ID, "ipv4", netip.MustParseAddr("10.77.0.100")))

	static, err := d.GetStaticLease(ctx, dn.ID, "ipv4", staticIMSI)
	g.must("GetStaticLease", err)
	g.must("UpdateStaticLeaseAddress", d.UpdateStaticLeaseAddress(ctx, static.ID, netip.MustParseAddr("10.77.0.101")))
	g.must("DeleteStaticLease", d.DeleteStaticLease(ctx, static.ID))

	g.must("ReplaceFramedRoutes", d.ReplaceFramedRoutes(ctx, imsi, dn.ID, []netip.Prefix{netip.MustParsePrefix("10.200.0.0/24"), netip.MustParsePrefix("10.201.0.0/24")}))

	userID, err := d.CreateUser(ctx, &User{Email: "golden@example.com", RoleID: RoleAdmin, HashedPassword: "hash-1"})
	g.must("CreateUser", err)

	_, err = d.CreateUser(ctx, &User{Email: "second@example.com", RoleID: RoleAdmin, HashedPassword: "hash-2"})
	g.must("CreateUser second", err)
	g.must("UpdateUser", d.UpdateUser(ctx, "second@example.com", RoleReadOnly))
	g.must("UpdateUserPassword", d.UpdateUserPassword(ctx, "second@example.com", "hash-3"))

	expires := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
	g.must("CreateAPIToken", d.CreateAPIToken(ctx, &APIToken{ID: "01890000-0000-7000-8000-00000000c001", TokenID: "golden-token", Name: "golden", TokenHash: "hash", UserID: userID, ExpiresAt: &expires}))
	g.must("DeleteAPIToken", d.DeleteAPIToken(ctx, "01890000-0000-7000-8000-00000000c001"))

	for i := range 6 {
		g.must("CreateSession", d.CreateSession(ctx, &Session{ID: fmt.Sprintf("01890000-0000-7000-8000-%012d", 900+i), UserID: userID, TokenHash: []byte{byte(i)}, CreatedAt: 1767225600, ExpiresAt: 4102444800, Method: SessionPassword}))
	}

	g.must("DeleteOldestSessions", d.DeleteOldestSessions(ctx, userID, 2))
	g.must("DeleteSessionByTokenHash", d.DeleteSessionByTokenHash(ctx, []byte{5}))

	_, err = d.DeleteExpiredSessions(ctx)
	g.must("DeleteExpiredSessions", err)
	g.must("DeleteAllSessionsForUser", d.DeleteAllSessionsForUser(ctx, userID))
	g.must("CreateSession again", d.CreateSession(ctx, &Session{ID: "01890000-0000-7000-8000-000000000999", UserID: userID, TokenHash: []byte{9}, CreatedAt: 1767225600, ExpiresAt: 4102444800, Method: SessionPassword}))
	g.must("DeleteAllSessions", d.DeleteAllSessions(ctx))

	g.must("UpdateOIDC", d.UpdateOIDC(ctx, OIDC{Enabled: true, ExternalURL: "https://core.example.com", Issuer: "https://idp.example.com", ClientID: "core", ClientSecret: "secret", DisplayName: "IdP", Scopes: []string{"openid", "email"}, SessionLifetime: time.Hour, GroupRoles: []GroupRole{{Group: "admins", RoleID: RoleAdmin}}}))
	g.must("UseOIDCState", d.UseOIDCState(ctx, bytes.Repeat([]byte{3}, 32), 4102444800, 1767225600))
	g.must("LinkOIDCIdentity", d.LinkOIDCIdentity(ctx, &OIDCLink{Issuer: "https://idp.example.com", ClientID: "core", Subject: "sub-1", Email: "golden@example.com", UserID: userID, IdentityID: "01890000-0000-7000-8000-00000000d001", At: 1767225600}))

	_, err = d.LogInWithOIDC(ctx, &OIDCLogin{
		Issuer: "https://idp.example.com", ClientID: "core", Subject: "sub-2", Email: "new@example.com", LinkEmail: "new@example.com", MapsGroups: true, RoleID: RoleAdmin, MaxUsers: 100, MaxSessionsPerUser: 5, NewUserID: "01890000-0000-7000-8000-00000000d002", NewIdentityID: "01890000-0000-7000-8000-00000000d003",
		Session: Session{ID: "01890000-0000-7000-8000-00000000d004", TokenHash: []byte{42}, CreatedAt: 1767225600, ExpiresAt: 4102444800, Method: SessionOIDC},
	})
	g.must("LogInWithOIDC", err)
	g.must("UnlinkUser", d.UnlinkUser(ctx, "golden@example.com"))
	g.must("DeleteUser", d.DeleteUser(ctx, "second@example.com"))

	cell := &CellPosition{ID: "golden-cell", RAT: RATNR, Mcc: "001", Mnc: "01", CellIdentity: "00066c000", Latitude: 45, Longitude: 21.5, Source: "manual"}
	g.must("CreateCellPosition", d.CreateCellPosition(ctx, cell))

	cell.Latitude = 46
	g.must("UpdateCellPosition", d.UpdateCellPosition(ctx, cell))
	g.must("DeleteCellPosition", d.DeleteCellPosition(ctx, cell.ID))

	g.must("UpsertClusterMember", d.UpsertClusterMember(ctx, &ClusterMember{NodeID: "2", APIAddress: "https://10.0.0.2:5002", BinaryVersion: "v1"}))
	g.must("UpsertClusterMember 3", d.UpsertClusterMember(ctx, &ClusterMember{NodeID: "3", APIAddress: "https://10.0.0.3:5002", BinaryVersion: "v1"}))
	g.must("SetDisplayName", d.SetDisplayName(ctx, "2", "node two"))

	_, err = d.SetDrainState(ctx, "2", DrainStateDraining)
	g.must("SetDrainState", err)

	g.must("InitClusterJoinHMACKey", d.InitClusterJoinHMACKey(ctx, bytes.Repeat([]byte{1}, 32)))
	g.must("MintJoinTokenRecord", d.MintJoinTokenRecord(ctx, &ClusterJoinToken{ID: "token-a", ClaimsJSON: "{}", ExpiresAt: 4102444800}))
	g.must("MintJoinTokenRecord b", d.MintJoinTokenRecord(ctx, &ClusterJoinToken{ID: "token-b", ClaimsJSON: "{}", ExpiresAt: 4102444800}))
	g.must("MintJoinTokenRecord stale", d.MintJoinTokenRecord(ctx, &ClusterJoinToken{ID: "token-stale", ClaimsJSON: "{}", ExpiresAt: 1000}))
	g.must("ConsumeJoinToken", d.ConsumeJoinToken(ctx, "token-a", "3"))

	_, err = d.RedeemJoinToken(ctx, "token-b", "4", "sha256:44", "PEM-4")
	g.must("RedeemJoinToken", err)
	g.must("UpsertClusterNodeCert", d.UpsertClusterNodeCert(ctx, &ClusterNodeCert{NodeID: "5", Fingerprint: "sha256:55", CertPEM: "PEM-5", AddedAt: 1767225600}))
	g.must("DeleteClusterNodeCert", d.DeleteClusterNodeCert(ctx, "5"))
	g.must("DeleteStaleJoinTokens", d.DeleteStaleJoinTokens(ctx, time.Unix(2000, 0)))

	_, err = d.RegisterUE(ctx, imsi, UERegistrationTypeMME, "1", "")
	g.must("RegisterUE", err)

	_, err = d.RegisterUE(ctx, staticIMSI, UERegistrationTypeMME, "3", "")
	g.must("RegisterUE on node 3", err)
	g.must("PurgeUERegistration", d.PurgeUERegistration(ctx, imsi, UERegistrationTypeMME, "1"))

	g.must("UpdateSMSSettings", d.UpdateSMSSettings(ctx, &SMSSettings{SMSNumber: "15550001"}))
	g.must("CreateSMSC", d.CreateSMSC(ctx, &SMSC{ID: "golden-smsc-id", Name: "golden-smsc", ServiceCentres: []string{"15550002"}, Nodes: []SMSCNode{{ID: "golden-node", DiameterIdentity: "smsc.example.org", Address: "10.0.0.9", Port: 3868}}}))

	smsc, err := d.GetSMSC(ctx, "golden-smsc")
	g.must("GetSMSC", err)

	smsc.ServiceCentres = []string{"15550003"}
	g.must("UpdateSMSC", d.UpdateSMSC(ctx, smsc))
	g.must("RecordSMSWaiting", d.RecordSMSWaiting(ctx, SMSWaitingUpdate{IMSI: imsi, ServiceCentre: "15550003", MemoryFull: true}))
	g.must("ClearSMSMemoryFull", d.ClearSMSMemoryFull(ctx, imsi))
	g.must("RemoveSMSWaitingCentre", d.RemoveSMSWaitingCentre(ctx, imsi, "15550003"))
	g.must("RecordSMSWaiting again", d.RecordSMSWaiting(ctx, SMSWaitingUpdate{IMSI: imsi, ServiceCentre: "15550004"}))
	g.must("DeleteSMSWaiting", d.DeleteSMSWaiting(ctx, imsi))
	g.must("RecordSMSWaiting stale", d.RecordSMSWaiting(ctx, SMSWaitingUpdate{IMSI: staticIMSI, ServiceCentre: "15550004"}))
	g.must("DeleteStaleSMSWaiting", d.DeleteStaleSMSWaiting(ctx, 365*24*time.Hour))
	g.must("DeleteSMSC", d.DeleteSMSC(ctx, smsc.ID))

	for _, op := range []string{"CreateSMSCPeer", "UpdateSMSCPeer"} {
		if err := g.raw(op, &legacySMSCPeer{ID: "legacy", DiameterIdentity: "legacy.example.org", Address: "10.0.0.10", Port: 3868, ServiceCentres: []string{"15550005"}}); !errors.Is(err, ErrRetiredOperation) {
			t.Fatalf("%s: err = %v, want ErrRetiredOperation", op, err)
		}
	}

	if err := g.raw("DeleteSMSCPeer", &stringPayload{Value: "legacy"}); !errors.Is(err, ErrRetiredOperation) {
		t.Fatalf("DeleteSMSCPeer: err = %v, want ErrRetiredOperation", err)
	}

	g.must("UpdateIMSConfiguration", d.UpdateIMSConfiguration(ctx, &IMSSettings{HomeDomain: "ims.example.org", AdvertiseVoice: true}, []string{"10.0.0.20"}))
	g.must("ReplacePCSCFAddresses", g.raw("ReplacePCSCFAddresses", &pcscfAddressesPayload{Addresses: []string{"10.0.0.21", "10.0.0.22"}}))

	peer := &IMSPeer{ID: "golden-peer-id", Name: "golden-peer", OriginHost: "scscf.ims.example.org", Realm: "ims.example.org", Addresses: []string{"10.0.0.30"}}
	g.must("CreateIMSPeer", d.CreateIMSPeer(ctx, peer))

	peers, err := d.ListIMSPeers(ctx)
	g.must("ListIMSPeers", err)

	peer = &peers[0]
	peer.Addresses = []string{"10.0.0.31"}
	g.must("UpdateIMSPeer", d.UpdateIMSPeer(ctx, peer))
	g.must("DeleteIMSPeer", d.DeleteIMSPeer(ctx, peer.ID))

	g.must("CreateIMSApplicationServer", d.CreateIMSApplicationServer(ctx, &IMSApplicationServer{ID: "golden-as-id", Name: "golden-as", ServerName: "sip:as.ims.example.org", PublicServiceIdentities: []string{"sip:conf@ims.example.org"}}))
	g.must("UpdateIMSApplicationServer", d.UpdateIMSApplicationServer(ctx, &IMSApplicationServer{ID: "golden-as-id", Name: "golden-as", ServerName: "sip:as2.ims.example.org", PublicServiceIdentities: []string{"sip:conf@ims.example.org"}}))
	g.must("CreateIMSServiceProfile", d.CreateIMSServiceProfile(ctx, &IMSServiceProfile{ID: "golden-sp-id", Name: "golden-sp"}))
	g.must("UpdateIMSServiceProfile", d.UpdateIMSServiceProfile(ctx, &IMSServiceProfile{ID: "golden-sp-id", Name: "golden-sp"}))

	sp, err := d.GetIMSServiceProfile(ctx, "golden-sp")
	g.must("GetIMSServiceProfile", err)
	g.must("SetProfileIMSServiceProfiles", d.SetProfileIMSServiceProfiles(ctx, profile.ID, map[IMSRegistrationSet]string{IMSRegistrationSetMMTel: sp.ID}))
	g.must("SetProfileIMSServiceProfiles clear", d.SetProfileIMSServiceProfiles(ctx, profile.ID, map[IMSRegistrationSet]string{}))
	g.must("DeleteIMSServiceProfile", d.DeleteIMSServiceProfile(ctx, "golden-sp"))
	g.must("DeleteIMSApplicationServer", d.DeleteIMSApplicationServer(ctx, "golden-as"))

	pending := &IMSRegistration{
		ServerName: "sip:scscf.ims.example.org", OriginHost: "scscf.ims.example.org", OriginRealm: "ims.example.org",
		Sets: map[IMSRegistrationSet]IMSSetRegistration{IMSRegistrationSetMMTel: {State: IMSNotRegistered, AuthPending: true}},
	}

	_, swapped, err := d.CompareAndSwapIMSRegistration(ctx, imsi, nil, pending)
	g.must("CompareAndSwapIMSRegistration", err)

	if !swapped {
		t.Fatal("CompareAndSwapIMSRegistration did not swap")
	}

	g.must("PutIPSMGWRegistration", d.PutIPSMGWRegistration(ctx, IPSMGWRegistration{IMSI: imsi, Number: "15550006", DiameterHost: "ipsmgw.example.org", DiameterRealm: "example.org"}))
	g.must("DeleteIPSMGWRegistration", d.DeleteIPSMGWRegistration(ctx, imsi))

	g.must("CreateRoamingDiameterPeer", d.CreateRoamingDiameterPeer(ctx, &RoamingDiameterPeer{ID: "golden-dea", DiameterIdentity: "dea.example.org", Address: "10.0.0.40", Port: 3868}))
	g.must("UpdateRoamingDiameterPeer", d.UpdateRoamingDiameterPeer(ctx, &RoamingDiameterPeer{ID: "golden-dea", DiameterIdentity: "dea.example.org", Address: "10.0.0.41", Port: 3868}))

	peerID := "golden-dea"
	g.must("CreateRoamingPartner", d.CreateRoamingPartner(ctx, &RoamingPartner{ID: "golden-partner", Name: "partner", PLMNs: []RoamingPLMN{{Mcc: "208", Mnc: "93"}}, ProfileID: profile.ID, DiameterPeerID: &peerID}))
	g.must("UpdateRoamingPartner", d.UpdateRoamingPartner(ctx, &RoamingPartner{ID: "golden-partner", Name: "partner-2", PLMNs: []RoamingPLMN{{Mcc: "208", Mnc: "93"}}, ProfileID: profile.ID, DiameterPeerID: &peerID}))

	_, err = d.RegisterVisitorUE(ctx, visitorIMSI, "golden-partner", UERegistrationTypeMME, "1", "")
	g.must("RegisterVisitorUE", err)
	g.must("SetVisitorMSISDN", d.SetVisitorMSISDN(ctx, visitorIMSI, "33600000001"))
	g.must("SetVisitorDeregistering", d.SetVisitorDeregistering(ctx, visitorIMSI, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)))
	g.must("SetRoamingPartnerWithdrawing", d.SetRoamingPartnerWithdrawing(ctx, "golden-partner", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)))

	g.must("PutGeneratedSEPPIdentity", d.PutGeneratedSEPPIdentity(ctx, 1))

	identity, err := d.GetSEPPIdentity(ctx, 1)
	g.must("GetSEPPIdentity", err)

	identity.Source = SEPPIdentityImported
	g.must("PutSEPPIdentity", d.PutSEPPIdentity(ctx, identity))
	g.must("DeleteSEPPIdentity", d.DeleteSEPPIdentity(ctx, 1))

	_, err = d.RegenerateSEPPCA(ctx)
	g.must("RegenerateSEPPCA", err)

	g.must("DeleteClusterMember", d.DeleteClusterMember(ctx, "3"))

	_, err = opVerifyState.Invoke(ctx, d, &struct{}{})
	g.must("VerifyState", err)

	g.must("DeletePolicy golden", d.DeletePolicy(ctx, "golden-policy"))
	g.must("DeleteSliceDataNetwork", d.DeleteSliceDataNetwork(ctx, slice.ID, dn.ID))
	g.must("DeleteNetworkSlice", d.DeleteNetworkSlice(ctx, "golden-slice"))

	other := &DataNetwork{Name: "golden-other", IPv4Pool: "10.78.0.0/24", DNS: "8.8.8.8", MTU: 1400}
	g.must("CreateDataNetwork other", d.CreateDataNetwork(ctx, other))
	g.must("DeleteDataNetwork", d.DeleteDataNetwork(ctx, "golden-other"))

	unused := &Profile{Name: "golden-unused", UeAmbrUplink: "1 Mbps", UeAmbrDownlink: "1 Mbps", Allow4G: true, Allow5G: true}
	g.must("CreateProfile unused", d.CreateProfile(ctx, unused))
	g.must("DeleteProfile", d.DeleteProfile(ctx, "golden-unused"))
	g.must("PurgeUERegistration visitor", d.PurgeUERegistration(ctx, visitorIMSI, UERegistrationTypeMME, "1"))
	g.must("DeleteRoamingPartner", d.DeleteRoamingPartner(ctx, "golden-partner"))
	g.must("DeleteRoamingDiameterPeer", d.DeleteRoamingDiameterPeer(ctx, "golden-dea"))

	grouped := entry(t, nextIndex(t, d), d.appliedLeaderTime.Load()+1,
		command(t, "SetRetentionPolicy", &RetentionPolicy{Category: CategoryAuditLogs, Days: 11}),
		command(t, "CreateProfile", &Profile{ID: profile.ID, Name: "golden-profile", UeAmbrUplink: "1 Mbps", UeAmbrDownlink: "1 Mbps", Allow4G: true, Allow5G: true}),
		command(t, "UpdateRadioEventSettings", &boolPayload{Value: true}),
	)

	out, err := d.ApplyEntries(ctx, []ellaraft.Entry{grouped})
	g.must("grouped entry", err)

	if results, _ := out[0].([]commandResult); len(results) != 3 || results[0].err != nil || !errors.Is(results[1].err, ErrAlreadyExists) || results[2].err != nil {
		t.Fatalf("grouped entry results = %+v, want success, ErrAlreadyExists, success", out[0])
	}
}

func goldenEntries(t *testing.T, database *Database) []goldenEntry {
	t.Helper()

	rec := testRecorder.recordingFor(database)
	if rec == nil {
		t.Fatal("the golden workload database was not recorded")
	}

	rec.mu.Lock()
	defer rec.mu.Unlock()

	var out []goldenEntry

	for _, call := range rec.calls {
		if call.err != "" {
			t.Fatalf("the golden workload produced a fatal apply error: %s", call.err)
		}

		for _, e := range call.entries {
			out = append(out, goldenEntry{Index: e.Index, Term: e.Term, Entry: json.RawMessage(e.Data)})
		}
	}

	return out
}

func readGoldenLog(t *testing.T) []goldenEntry {
	t.Helper()

	f, err := os.Open(goldenLogPath)
	if err != nil {
		t.Fatalf("open golden log: %v (regenerate with: go test ./internal/db/ -run TestGoldenCommandLog -update)", err)
	}

	defer func() { _ = f.Close() }()

	var out []goldenEntry

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 1<<24)

	for scanner.Scan() {
		var e goldenEntry
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			t.Fatalf("decode golden log line: %v", err)
		}

		out = append(out, e)
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("read golden log: %v", err)
	}

	return out
}

func writeGoldenLog(t *testing.T, entries []goldenEntry) {
	t.Helper()

	var buf bytes.Buffer

	for _, e := range entries {
		line, err := json.Marshal(e)
		if err != nil {
			t.Fatalf("encode golden entry: %v", err)
		}

		buf.Write(line)
		buf.WriteByte('\n')
	}

	if err := os.MkdirAll(filepath.Dir(goldenLogPath), 0o755); err != nil {
		t.Fatalf("create golden dir: %v", err)
	}

	if err := os.WriteFile(goldenLogPath, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write golden log: %v", err)
	}
}

func replayGoldenLog(t *testing.T, entries []goldenEntry) (string, map[string]int) {
	t.Helper()

	database := newDatabaseAtVersion(t, commandSchema)
	ctx := t.Context()

	var out strings.Builder

	succeeded := map[string]int{}

	for _, e := range entries {
		results, err := database.ApplyEntries(ctx, []ellaraft.Entry{{Index: e.Index, Term: e.Term, Data: e.Entry}})
		if err != nil {
			t.Fatalf("apply golden entry %d: %v", e.Index, err)
		}

		var batch commandBatch
		if err := json.Unmarshal(e.Entry, &batch); err != nil {
			t.Fatalf("decode golden entry %d: %v", e.Index, err)
		}

		encoded := encodeApplyResults(results)

		cmdResults, _ := results[0].([]commandResult)

		for i, c := range batch.Commands {
			if i < len(cmdResults) && cmdResults[i].err == nil {
				succeeded[c.Op]++
			}
		}

		fmt.Fprintf(&out, "entry %d %s\n", e.Index, strings.Join(goldenOps(batch), ","))

		for _, line := range strings.Split(encoded[0], "\n") {
			fmt.Fprintf(&out, "  -> %s\n", line)
		}

		hash, err := hashReplicatedRows(ctx, database.writer())
		if err != nil {
			t.Fatalf("hash after golden entry %d: %v", e.Index, err)
		}

		fmt.Fprintf(&out, "  state %s\n", hash)
	}

	out.WriteString(dumpReplicatedState(t, database))

	return out.String(), succeeded
}

func goldenOps(b commandBatch) []string {
	ops := make([]string, len(b.Commands))
	for i, c := range b.Commands {
		ops[i] = c.Op
	}

	return ops
}

func dumpReplicatedState(t *testing.T, database *Database) string {
	t.Helper()

	ctx := t.Context()

	var out strings.Builder

	for _, table := range slices.Sorted(slices.Values(replicatedTables)) {
		columns, orderBy, err := tableHashColumns(ctx, database.writer(), table)
		if err != nil {
			t.Fatalf("columns of %s: %v", table, err)
		}

		if len(columns) == 0 {
			continue
		}

		selected := make([]string, len(columns))
		for i, c := range columns {
			selected[i] = "+" + quoteIdentifier(c)
		}

		fmt.Fprintf(&out, "table %s (%s)\n", table, strings.Join(columns, ", "))

		rows, err := database.writer().QueryContext(ctx, "SELECT "+strings.Join(selected, ", ")+" FROM "+quoteIdentifier(table)+" ORDER BY "+strings.Join(orderBy, ", "))
		if err != nil {
			t.Fatalf("dump %s: %v", table, err)
		}

		values := make([]any, len(columns))
		targets := make([]any, len(columns))

		for i := range values {
			targets[i] = &values[i]
		}

		for rows.Next() {
			if err := rows.Scan(targets...); err != nil {
				t.Fatalf("scan %s: %v", table, err)
			}

			cells := make([]string, len(values))

			for i, v := range values {
				switch v := v.(type) {
				case nil:
					cells[i] = "NULL"
				case []byte:
					cells[i] = "x'" + hex.EncodeToString(v) + "'"
				case string:
					cells[i] = fmt.Sprintf("%q", v)
				default:
					cells[i] = fmt.Sprintf("%v", v)
				}
			}

			fmt.Fprintf(&out, "  %s\n", strings.Join(cells, " | "))
		}

		if err := rows.Err(); err != nil {
			t.Fatalf("dump %s: %v", table, err)
		}

		_ = rows.Close()
	}

	return out.String()
}

func commandOperations() []string {
	var names []string

	for name := range changesetOps {
		if _, ok := commandHandlerFor(name); ok {
			names = append(names, name)
		}
	}

	for name := range intentOps {
		if _, ok := commandHandlerFor(name); ok {
			names = append(names, name)
		}
	}

	slices.Sort(names)

	return names
}

func TestGoldenCommandLog(t *testing.T) {
	if *updateSchemaGolden {
		if _, err := os.Stat(goldenLogPath); err == nil {
			compareGoldenState(t)

			if t.Failed() {
				return
			}
		}

		database := newDatabaseAtVersion(t, commandSchema)
		runGoldenWorkload(t, database)

		entries := goldenEntries(t, database)
		writeGoldenLog(t, entries)

		state, _ := replayGoldenLog(t, entries)
		if err := os.WriteFile(goldenStatePath, []byte(state), 0o644); err != nil {
			t.Fatalf("write golden state: %v", err)
		}
	}

	compareGoldenState(t)
}

func compareGoldenState(t *testing.T) {
	t.Helper()

	entries := readGoldenLog(t)
	got, _ := replayGoldenLog(t, entries)

	want, err := os.ReadFile(goldenStatePath)
	if err != nil {
		t.Fatalf("read golden state: %v (regenerate with: go test ./internal/db/ -run TestGoldenCommandLog -update)", err)
	}

	if got == string(want) {
		return
	}

	gotLines, wantLines := strings.Split(got, "\n"), strings.Split(string(want), "\n")

	for i := range max(len(gotLines), len(wantLines)) {
		var g, w string
		if i < len(gotLines) {
			g = gotLines[i]
		}

		if i < len(wantLines) {
			w = wantLines[i]
		}

		if g != w {
			at := ""

			for j := i; j >= 0 && j < len(wantLines); j-- {
				if strings.HasPrefix(wantLines[j], "entry ") {
					at = wantLines[j]
					break
				}
			}

			t.Fatalf("replaying %s no longer gives %s; an apply function changed what it writes or returns for a command already in the log.\n"+
				"That is a consensus change: gate it behind a new schema version instead of changing the existing behaviour.\n"+
				"first difference at line %d, %s:\n  golden:   %s\n  replayed: %s", goldenLogPath, goldenStatePath, i+1, at, w, g)
		}
	}
}

func TestGoldenCommandLogCoversEveryCommand(t *testing.T) {
	_, succeeded := replayGoldenLog(t, readGoldenLog(t))

	logged := map[string]bool{}

	for _, e := range readGoldenLog(t) {
		var b commandBatch
		if err := json.Unmarshal(e.Entry, &b); err != nil {
			t.Fatalf("decode golden entry %d: %v", e.Index, err)
		}

		for _, c := range b.Commands {
			logged[c.Op] = true
		}
	}

	for _, name := range commandOperations() {
		if _, rejected := goldenRejectedOperations[name]; rejected {
			if !logged[name] {
				t.Errorf("operation %q is not in the golden command log; add it to runGoldenWorkload and regenerate with -update", name)
			}

			continue
		}

		if succeeded[name] == 0 {
			t.Errorf("operation %q never succeeds in the golden command log; add it to runGoldenWorkload and regenerate with: go test ./internal/db/ -run TestGoldenCommandLog -update", name)
		}
	}
}

func goldenStateHashes(t *testing.T, golden string) map[uint64]string {
	t.Helper()

	out := map[uint64]string{}

	var index uint64

	for _, line := range strings.Split(golden, "\n") {
		if rest, ok := strings.CutPrefix(line, "entry "); ok {
			if _, err := fmt.Sscanf(rest, "%d", &index); err != nil {
				t.Fatalf("parse golden line %q: %v", line, err)
			}

			continue
		}

		if hash, ok := strings.CutPrefix(line, "  state "); ok {
			out[index] = hash
		}
	}

	return out
}

func TestGoldenCommandLogReplaysUnderPerturbations(t *testing.T) {
	entries := readGoldenLog(t)

	golden, err := os.ReadFile(goldenStatePath)
	if err != nil {
		t.Fatalf("read golden state: %v", err)
	}

	want := goldenStateHashes(t, string(golden))

	for _, pass := range replayFullPasses {
		if pass.timezone != "" {
			continue
		}

		t.Run(pass.name, func(t *testing.T) {
			ctx := t.Context()
			database := newDatabaseAtVersion(t, commandSchema)

			if pass.pragma != "" {
				if _, err := database.writer().ExecContext(ctx, pass.pragma); err != nil {
					t.Fatalf("%s: %v", pass.pragma, err)
				}
			}

			cycle := 0

			for start := 0; start < len(entries); {
				size := min(pass.groupSizes[cycle%len(pass.groupSizes)], len(entries)-start)
				cycle++

				if pass.physEvery > 0 {
					if err := replayReversePhysicalOrder(ctx, database); err != nil {
						t.Fatalf("reverse physical order: %v", err)
					}
				}

				group := make([]ellaraft.Entry, size)
				for i, e := range entries[start : start+size] {
					group[i] = ellaraft.Entry{Index: e.Index, Term: e.Term, Data: e.Entry}
				}

				if _, err := database.ApplyEntries(ctx, group); err != nil {
					t.Fatalf("apply golden entries %d..%d: %v", group[0].Index, group[size-1].Index, err)
				}

				last := group[size-1].Index

				got, err := hashReplicatedRows(ctx, database.writer())
				if err != nil {
					t.Fatalf("hash: %v", err)
				}

				if got != want[last] {
					var ops []string

					for _, e := range entries[start : start+size] {
						var b commandBatch

						_ = json.Unmarshal(e.Entry, &b)
						ops = append(ops, fmt.Sprintf("%d:%s", e.Index, strings.Join(goldenOps(b), "+")))
					}

					t.Fatalf("replicated state after entry %d differs from %s under %s: %s", last, goldenStatePath, pass.name, strings.Join(ops, ", "))
				}

				start += size
			}
		})
	}
}
