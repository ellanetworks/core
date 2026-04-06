// Copyright 2026 Ella Networks

package amf_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/db"
)

// configTestDB implements amf.DBer for GetSubscriberProfile / GetOperatorInfo tests.
type configTestDB struct {
	subscriber *db.Subscriber
	subErr     error
	policies   []db.Policy
	polErr     error
	slices     map[int]*db.NetworkSlice
	sliceErr   error
	allSlices  []db.NetworkSlice
	operator   *db.Operator
	opErr      error
}

func (d *configTestDB) GetOperator(context.Context) (*db.Operator, error) {
	return d.operator, d.opErr
}

func (d *configTestDB) GetSubscriber(_ context.Context, _ string) (*db.Subscriber, error) {
	return d.subscriber, d.subErr
}

func (d *configTestDB) GetDataNetworkByID(context.Context, int) (*db.DataNetwork, error) {
	return nil, nil
}

func (d *configTestDB) GetNetworkSliceByID(_ context.Context, id int) (*db.NetworkSlice, error) {
	s, ok := d.slices[id]
	if !ok {
		return nil, fmt.Errorf("slice %d not found", id)
	}

	return s, nil
}

func (d *configTestDB) GetProfileByID(_ context.Context, id int) (*db.Profile, error) {
	return &db.Profile{ID: id}, nil
}

func (d *configTestDB) GetPolicyByProfileAndSlice(context.Context, int, int) (*db.Policy, error) {
	return nil, nil
}

func (d *configTestDB) ListAllNetworkSlices(context.Context) ([]db.NetworkSlice, error) {
	if d.allSlices != nil {
		return d.allSlices, nil
	}

	// Build from the slices map so GetSubscriberProfile tests work.
	var out []db.NetworkSlice
	for _, s := range d.slices {
		out = append(out, *s)
	}

	return out, d.sliceErr
}

func (d *configTestDB) ListPoliciesByProfile(_ context.Context, _ int) ([]db.Policy, error) {
	return d.policies, d.polErr
}

func mustSUPI(t *testing.T) etsi.SUPI {
	t.Helper()

	supi, err := etsi.NewSUPIFromIMSI("001010000000001")
	if err != nil {
		t.Fatalf("invalid IMSI: %v", err)
	}

	return supi
}

func TestGetSubscriberProfile_SinglePolicy(t *testing.T) {
	sd := "010203"
	fakeDB := &configTestDB{
		subscriber: &db.Subscriber{ID: 1, Imsi: "001010000000001", ProfileID: 10},
		policies:   []db.Policy{{ID: 1, ProfileID: 10, SliceID: 100}},
		slices:     map[int]*db.NetworkSlice{100: {ID: 100, Name: "slice-a", Sst: 1, Sd: &sd}},
	}

	amfInstance := amf.New(fakeDB, nil, nil)

	profile, err := amfInstance.GetSubscriberProfile(context.Background(), mustSUPI(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(profile.AllowedNssai) != 1 {
		t.Fatalf("expected 1 NSSAI, got %d", len(profile.AllowedNssai))
	}

	if profile.AllowedNssai[0].Sst != 1 || profile.AllowedNssai[0].Sd != "010203" {
		t.Fatalf("expected {Sst:1, Sd:010203}, got %+v", profile.AllowedNssai[0])
	}

	if profile.Ambr == nil {
		t.Fatal("expected non-nil Ambr")
	}
}

func TestGetSubscriberProfile_MultiplePoliciesDifferentSlices(t *testing.T) {
	sd1 := "010203"
	sd2 := "aabbcc"
	fakeDB := &configTestDB{
		subscriber: &db.Subscriber{ID: 1, Imsi: "001010000000001", ProfileID: 10},
		policies: []db.Policy{
			{ID: 1, ProfileID: 10, SliceID: 100},
			{ID: 2, ProfileID: 10, SliceID: 200},
		},
		slices: map[int]*db.NetworkSlice{
			100: {ID: 100, Name: "slice-a", Sst: 1, Sd: &sd1},
			200: {ID: 200, Name: "slice-b", Sst: 2, Sd: &sd2},
		},
	}

	amfInstance := amf.New(fakeDB, nil, nil)

	profile, err := amfInstance.GetSubscriberProfile(context.Background(), mustSUPI(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(profile.AllowedNssai) != 2 {
		t.Fatalf("expected 2 NSSAIs, got %d", len(profile.AllowedNssai))
	}

	// Verify both slices are present
	found := map[int32]bool{}
	for _, s := range profile.AllowedNssai {
		found[s.Sst] = true
	}

	if !found[1] || !found[2] {
		t.Fatalf("expected SST 1 and 2, got %+v", profile.AllowedNssai)
	}
}

func TestGetSubscriberProfile_DeduplicatesSameSlice(t *testing.T) {
	sd := "010203"
	fakeDB := &configTestDB{
		subscriber: &db.Subscriber{ID: 1, Imsi: "001010000000001", ProfileID: 10},
		policies: []db.Policy{
			{ID: 1, ProfileID: 10, SliceID: 100},
			{ID: 2, ProfileID: 10, SliceID: 100}, // same slice
		},
		slices: map[int]*db.NetworkSlice{
			100: {ID: 100, Name: "slice-a", Sst: 1, Sd: &sd},
		},
	}

	amfInstance := amf.New(fakeDB, nil, nil)

	profile, err := amfInstance.GetSubscriberProfile(context.Background(), mustSUPI(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(profile.AllowedNssai) != 1 {
		t.Fatalf("expected 1 NSSAI after dedup, got %d", len(profile.AllowedNssai))
	}
}

func TestGetSubscriberProfile_NilSD(t *testing.T) {
	fakeDB := &configTestDB{
		subscriber: &db.Subscriber{ID: 1, Imsi: "001010000000001", ProfileID: 10},
		policies:   []db.Policy{{ID: 1, ProfileID: 10, SliceID: 100}},
		slices:     map[int]*db.NetworkSlice{100: {ID: 100, Name: "slice-a", Sst: 1, Sd: nil}},
	}

	amfInstance := amf.New(fakeDB, nil, nil)

	profile, err := amfInstance.GetSubscriberProfile(context.Background(), mustSUPI(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(profile.AllowedNssai) != 1 {
		t.Fatalf("expected 1 NSSAI, got %d", len(profile.AllowedNssai))
	}

	if profile.AllowedNssai[0].Sd != "" {
		t.Fatalf("expected empty SD for nil slice SD, got %q", profile.AllowedNssai[0].Sd)
	}
}

func TestGetSubscriberProfile_NoPolicies(t *testing.T) {
	fakeDB := &configTestDB{
		subscriber: &db.Subscriber{ID: 1, Imsi: "001010000000001", ProfileID: 10},
		policies:   []db.Policy{},
		slices:     map[int]*db.NetworkSlice{},
	}

	amfInstance := amf.New(fakeDB, nil, nil)

	profile, err := amfInstance.GetSubscriberProfile(context.Background(), mustSUPI(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(profile.AllowedNssai) != 0 {
		t.Fatalf("expected 0 NSSAIs for subscriber with no policies, got %d", len(profile.AllowedNssai))
	}
}

func TestGetSubscriberProfile_SubscriberNotFound(t *testing.T) {
	fakeDB := &configTestDB{
		subErr: fmt.Errorf("subscriber not found"),
	}

	amfInstance := amf.New(fakeDB, nil, nil)

	_, err := amfInstance.GetSubscriberProfile(context.Background(), mustSUPI(t))
	if err == nil {
		t.Fatal("expected error for missing subscriber, got nil")
	}
}

func TestGetSubscriberProfile_PolicyListError(t *testing.T) {
	fakeDB := &configTestDB{
		subscriber: &db.Subscriber{ID: 1, Imsi: "001010000000001", ProfileID: 10},
		polErr:     fmt.Errorf("db error"),
	}

	amfInstance := amf.New(fakeDB, nil, nil)

	_, err := amfInstance.GetSubscriberProfile(context.Background(), mustSUPI(t))
	if err == nil {
		t.Fatal("expected error for policy list failure, got nil")
	}
}

func TestListOperatorSnssai_MultipleSlices(t *testing.T) {
	sd1 := "010203"
	sd2 := "aabbcc"
	fakeDB := &configTestDB{
		operator: &db.Operator{
			Mcc:           "001",
			Mnc:           "01",
			SupportedTACs: "[\"000001\"]",
		},
		allSlices: []db.NetworkSlice{
			{ID: 1, Name: "slice-a", Sst: 1, Sd: &sd1},
			{ID: 2, Name: "slice-b", Sst: 2, Sd: &sd2},
		},
	}

	amfInstance := amf.New(fakeDB, nil, nil)

	snssaiList, err := amfInstance.ListOperatorSnssai(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(snssaiList) != 2 {
		t.Fatalf("expected 2 SNssai in list, got %d", len(snssaiList))
	}

	if snssaiList[0].Sst != 1 || snssaiList[0].Sd != "010203" {
		t.Fatalf("unexpected first SNSSAI: %+v", snssaiList[0])
	}

	if snssaiList[1].Sst != 2 || snssaiList[1].Sd != "aabbcc" {
		t.Fatalf("unexpected second SNSSAI: %+v", snssaiList[1])
	}
}

func TestListOperatorSnssai_SliceWithNilSD(t *testing.T) {
	fakeDB := &configTestDB{
		allSlices: []db.NetworkSlice{
			{ID: 1, Name: "default", Sst: 1, Sd: nil},
		},
	}

	amfInstance := amf.New(fakeDB, nil, nil)

	snssaiList, err := amfInstance.ListOperatorSnssai(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(snssaiList) != 1 {
		t.Fatalf("expected 1 SNssai, got %d", len(snssaiList))
	}

	if snssaiList[0].Sd != "" {
		t.Fatalf("expected empty SD, got %q", snssaiList[0].Sd)
	}
}
