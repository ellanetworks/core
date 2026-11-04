// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/pki"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func generatedSEPPIdentity(t *testing.T) *SEPPIdentity {
	t.Helper()

	plmn := pki.PLMN{MCC: InitialMcc, MNC: InitialMnc}

	leaf, key, ca, err := pki.GenerateSEPPCert(plmn.DefaultSEPPFQDN(), plmn)
	if err != nil {
		t.Fatalf("GenerateSEPPCert: %v", err)
	}

	keyPEM, err := pki.EncodePrivateKeyPEM(key)
	if err != nil {
		t.Fatalf("EncodePrivateKeyPEM: %v", err)
	}

	return &SEPPIdentity{SEPPID: 1, Source: SEPPIdentityImported, FQDN: plmn.DefaultSEPPFQDN(), CertChainPEM: string(pki.EncodeCertPEM(leaf)) + string(pki.EncodeCertPEM(ca)), KeyPEM: string(keyPEM)}
}

func TestSubscriberReadsAndWritesBeforeV26(t *testing.T) {
	ctx := context.Background()
	d := newDatabaseAtVersion(t, 25)

	if err := d.InitializeLocalSettings(ctx); err != nil {
		t.Fatalf("initialize local settings: %v", err)
	}

	if err := d.Initialize(ctx); err != nil {
		t.Fatalf("Initialize at schema 25: %v", err)
	}

	profile, err := d.GetProfile(ctx, InitialProfileName)
	if err != nil {
		t.Fatalf("get default profile: %v", err)
	}

	imsi := "001010000000001"

	sub := &Subscriber{
		Imsi:           imsi,
		SequenceNumber: "000000000001",
		PermanentKey:   "00112233445566778899aabbccddeeff",
		Opc:            "00112233445566778899aabbccddeeff",
		ProfileID:      profile.ID,
		Msisdn:         "15550001",
	}

	if err := d.CreateSubscriber(ctx, sub); err != nil {
		t.Fatalf("CreateSubscriber at schema 25: %v", err)
	}

	if got, err := d.GetSubscriber(ctx, imsi); err != nil || got.Msisdn != "15550001" || got.RoamingPartnerID != nil {
		t.Fatalf("GetSubscriber at schema 25 = %+v, %v", got, err)
	}

	if got, err := d.GetSubscriberOrVisitor(ctx, imsi); err != nil || got.Imsi != imsi {
		t.Fatalf("GetSubscriberOrVisitor at schema 25 = %+v, %v", got, err)
	}

	if got, err := d.GetSubscriberByMSISDN(ctx, "15550001"); err != nil || got.Imsi != imsi {
		t.Fatalf("GetSubscriberByMSISDN at schema 25 = %+v, %v", got, err)
	}

	search := "0001"

	if subs, total, err := d.ListSubscribersPage(ctx, &SubscriberFilters{Search: &search}, 1, 10); err != nil || total != 1 || len(subs) != 1 {
		t.Fatalf("ListSubscribersPage at schema 25 = %v, %d, %v", subs, total, err)
	}

	roaming := true

	if subs, total, err := d.ListSubscribersPage(ctx, &SubscriberFilters{Roaming: &roaming}, 1, 10); err != nil || total != 0 || len(subs) != 0 {
		t.Fatalf("ListSubscribersPage of roamers at schema 25 = %v, %d, %v", subs, total, err)
	}

	if n, err := d.CountSubscribers(ctx); err != nil || n != 1 {
		t.Fatalf("CountSubscribers at schema 25 = %d, %v", n, err)
	}

	if n, err := d.CountSubscribersAndVisitors(ctx); err != nil || n != 1 {
		t.Fatalf("CountSubscribersAndVisitors at schema 25 = %d, %v", n, err)
	}

	if n, err := d.CountSubscribersInProfile(ctx, profile.ID); err != nil || n != 1 {
		t.Fatalf("CountSubscribersInProfile at schema 25 = %d, %v", n, err)
	}

	if _, err := d.AdvanceSubscriberSQN(ctx, imsi, "", ""); err != nil {
		t.Fatalf("AdvanceSubscriberSQN at schema 25: %v", err)
	}

	if _, err := d.GetUsagePerSubscriber(ctx, "", NewDayRange(time.Now().AddDate(0, 0, -1), time.Now()), nil, NoUsageLimit); err != nil {
		t.Fatalf("GetUsagePerSubscriber at schema 25: %v", err)
	}

	if err := d.IncrementDailyUsage(ctx, DailyUsage{EpochDay: DaysSinceEpoch(time.Now()), IMSI: imsi, BytesUplink: 1, BytesDownlink: 2}); err != nil {
		t.Fatalf("IncrementDailyUsage at schema 25: %v", err)
	}

	if perDay, err := d.GetUsagePerDay(ctx, "", NewDayRange(time.Now().AddDate(0, 0, -1), time.Now()), nil); err != nil || len(perDay) != 1 || perDay[0].BytesUplink != 1 {
		t.Fatalf("GetUsagePerDay at schema 25 = %+v, %v", perDay, err)
	}

	home, roamers := false, true

	if perDay, err := d.GetUsagePerDay(ctx, "", NewDayRange(time.Now().AddDate(0, 0, -1), time.Now()), &home); err != nil || len(perDay) != 1 {
		t.Fatalf("GetUsagePerDay of home subscribers at schema 25 = %+v, %v", perDay, err)
	}

	if perDay, err := d.GetUsagePerDay(ctx, "", NewDayRange(time.Now().AddDate(0, 0, -1), time.Now()), &roamers); err != nil || len(perDay) != 0 {
		t.Fatalf("GetUsagePerDay of roamers at schema 25 = %+v, %v", perDay, err)
	}

	if perSub, err := d.GetUsagePerSubscriber(ctx, "", NewDayRange(time.Now().AddDate(0, 0, -1), time.Now()), &roamers, NoUsageLimit); err != nil || len(perSub) != 0 {
		t.Fatalf("GetUsagePerSubscriber of roamers at schema 25 = %+v, %v", perSub, err)
	}

	if _, err := d.ListSubscriberIMSFilterCriteria(ctx, imsi, IMSRegistrationSetMMTel); err != nil {
		t.Fatalf("ListSubscriberIMSFilterCriteria at schema 25: %v", err)
	}

	if _, err := d.IMSRegisteredSubscribersOfProfiles(ctx, []string{profile.ID}); err != nil {
		t.Fatalf("IMSRegisteredSubscribersOfProfiles at schema 25: %v", err)
	}

	if _, err := d.GetSessionPolicy(ctx, imsi, InitialSliceSst, "", InitialDataNetworkName); err != nil {
		t.Fatalf("GetSessionPolicy at schema 25: %v", err)
	}

	if err := d.DeleteOldDailyUsage(ctx, 30); err != nil {
		t.Fatalf("DeleteOldDailyUsage at schema 25: %v", err)
	}

	sub.Description = "updated"
	if err := d.UpdateSubscriberProfile(ctx, sub); err != nil {
		t.Fatalf("UpdateSubscriberProfile at schema 25: %v", err)
	}

	if err := d.DeleteSubscriber(ctx, imsi); err != nil {
		t.Fatalf("DeleteSubscriber at schema 25: %v", err)
	}
}

func TestRoamingReadsAndWritesBeforeV26(t *testing.T) {
	ctx := context.Background()
	d := newDatabaseAtVersion(t, 25)

	if partners, err := d.ListRoamingPartners(ctx); err != nil || len(partners) != 0 {
		t.Fatalf("ListRoamingPartners at schema 25 = %v, %v", partners, err)
	}

	if _, err := d.GetRoamingPartner(ctx, "rp"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetRoamingPartner at schema 25: err = %v, want ErrNotFound", err)
	}

	if err := d.CreateSMSCPeer(ctx, &SMSCPeer{ID: "smsc", Address: "10.0.0.5", Port: 3868, ServiceCentres: []string{"15550000000"}}); err != nil {
		t.Fatalf("CreateSMSCPeer at schema 25: %v", err)
	}

	if peers, err := d.ListRoamingDiameterPeers(ctx); err != nil || len(peers) != 0 {
		t.Fatalf("ListRoamingDiameterPeers at schema 25 = %v, %v", peers, err)
	}

	if visitors, err := d.ListVisitors(ctx); err != nil || len(visitors) != 0 {
		t.Fatalf("ListVisitors at schema 25 = %v, %v", visitors, err)
	}

	if _, err := d.GetSEPPIdentity(ctx, 1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetSEPPIdentity at schema 25: err = %v, want ErrNotFound", err)
	}

	writes := map[string]func() error{
		"CreateRoamingPartner": func() error {
			return d.CreateRoamingPartner(ctx, &RoamingPartner{ID: "rp", Name: "partner", PLMNs: []RoamingPLMN{{Mcc: "208", Mnc: "93"}}, ProfileID: "pr"})
		},
		"CreateRoamingDiameterPeer": func() error {
			return d.CreateRoamingDiameterPeer(ctx, &RoamingDiameterPeer{ID: "hss", Address: "10.0.0.7", Port: 3868})
		},
		"PutSEPPIdentity": func() error {
			return d.PutSEPPIdentity(ctx, generatedSEPPIdentity(t))
		},
		"RegisterVisitorUE": func() error {
			_, err := d.RegisterVisitorUE(ctx, "208930000000001", "rp", UERegistrationTypeMME, "node-1", "")
			return err
		},
		"DeleteUnregisteredVisitors": func() error {
			_, err := d.DeleteUnregisteredVisitors(ctx)
			return err
		},
	}

	for name, write := range writes {
		if err := write(); !errors.Is(err, ErrMigrationPending) {
			t.Errorf("%s at schema 25: err = %v, want ErrMigrationPending", name, err)
		}
	}
}

func TestIncrementDailyUsageBatchBeforeV26ReportsTheUsageItCouldNotCharge(t *testing.T) {
	ctx := context.Background()
	d := newDatabaseAtVersion(t, 25)

	if err := d.InitializeLocalSettings(ctx); err != nil {
		t.Fatalf("initialize local settings: %v", err)
	}

	if err := d.Initialize(ctx); err != nil {
		t.Fatalf("Initialize at schema 25: %v", err)
	}

	profile, err := d.GetProfile(ctx, InitialProfileName)
	if err != nil {
		t.Fatalf("get default profile: %v", err)
	}

	if err := d.CreateSubscriber(ctx, &Subscriber{
		Imsi:           "001010000000001",
		SequenceNumber: "000000000001",
		PermanentKey:   "00112233445566778899aabbccddeeff",
		Opc:            "00112233445566778899aabbccddeeff",
		ProfileID:      profile.ID,
	}); err != nil {
		t.Fatalf("CreateSubscriber at schema 25: %v", err)
	}

	day := DaysSinceEpoch(time.Now())

	rows := []DailyUsage{
		{EpochDay: day, IMSI: "001019999999998", BytesUplink: 100, BytesDownlink: 200},
		{EpochDay: day, IMSI: "001019999999999", BytesUplink: 70, BytesDownlink: 20},
		{EpochDay: day, IMSI: "001010000000001", BytesUplink: 500, BytesDownlink: 300},
	}

	core, logs := observer.New(zapcore.ErrorLevel)
	t.Cleanup(logger.SwapSystemCore(core))

	if err := d.IncrementDailyUsageBatch(ctx, rows); err != nil {
		t.Fatalf("IncrementDailyUsageBatch at schema 25: %v", err)
	}

	entries := logs.FilterMessage("usage bytes lost: no subscriber row to charge").All()
	if len(entries) != 1 {
		t.Fatalf("got %d loss reports, want 1", len(entries))
	}

	fields := entries[0].ContextMap()
	if fields["rows"] != int64(2) || fields["uplink_volume"] != uint64(170) || fields["downlink_volume"] != uint64(220) {
		t.Errorf("loss report = %v, want 2 rows, 170 up, 220 down", fields)
	}

	perDay, err := d.GetUsagePerDay(ctx, "", NewDayRange(time.Now(), time.Now().Add(time.Hour)), nil)
	if err != nil || len(perDay) != 1 || perDay[0].BytesUplink != 500 {
		t.Fatalf("GetUsagePerDay at schema 25 = %+v, %v", perDay, err)
	}
}
