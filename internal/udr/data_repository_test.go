package udr_test

import (
	"errors"
	"testing"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/udr"
	"github.com/omec-project/openapi/models"
)

type MockDatabaseInstance struct {
	GetSubscriberFunc   func(ueId string) (*db.Subscriber, error)
	GetProfileByIDFunc  func(profileID int) (*db.Profile, error)
	assertedSubscribers map[string]struct{}
	assertedProfiles    map[int]struct{}
}

func (m *MockDatabaseInstance) GetSubscriber(ueId string) (*db.Subscriber, error) {
	m.assertedSubscribers[ueId] = struct{}{}
	return m.GetSubscriberFunc(ueId)
}

func (m *MockDatabaseInstance) GetProfileByID(profileID int) (*db.Profile, error) {
	m.assertedProfiles[profileID] = struct{}{}
	return m.GetProfileByIDFunc(profileID)
}

func (m *MockDatabaseInstance) UpdateSubscriber(subscriber *db.Subscriber) error {
	return nil
}

func NewMockDatabaseInstance() *MockDatabaseInstance {
	return &MockDatabaseInstance{
		assertedSubscribers: make(map[string]struct{}),
		assertedProfiles:    make(map[int]struct{}),
	}
}

func TestGetAmData_Success(t *testing.T) {
	mockDb := NewMockDatabaseInstance()
	mockDb.GetSubscriberFunc = func(ueId string) (*db.Subscriber, error) {
		if ueId == "test-ue" {
			return &db.Subscriber{ProfileID: 1}, nil
		}
		return nil, errors.New("subscriber not found")
	}
	mockDb.GetProfileByIDFunc = func(profileID int) (*db.Profile, error) {
		if profileID == 1 {
			return &db.Profile{BitrateDownlink: "100Mbps", BitrateUplink: "50Mbps"}, nil
		}
		return nil, errors.New("profile not found")
	}

	udr.NewUdrContext(1, mockDb)

	expectedData := &models.AccessAndMobilitySubscriptionData{
		Nssai: &models.Nssai{
			DefaultSingleNssais: []models.Snssai{{Sd: "1", Sst: 1}},
			SingleNssais:        []models.Snssai{{Sd: "1", Sst: 1}},
		},
		SubscribedUeAmbr: &models.AmbrRm{
			Downlink: "100Mbps",
			Uplink:   "50Mbps",
		},
	}

	amData, err := udr.GetAmData("test-ue")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if amData == nil {
		t.Fatal("expected amData to not be nil")
	}
	if amData.SubscribedUeAmbr.Downlink != expectedData.SubscribedUeAmbr.Downlink ||
		amData.SubscribedUeAmbr.Uplink != expectedData.SubscribedUeAmbr.Uplink {
		t.Errorf("expected AMBR %v, got %v", expectedData.SubscribedUeAmbr, amData.SubscribedUeAmbr)
	}

	if len(mockDb.assertedSubscribers) == 0 {
		t.Error("GetSubscriber was not called")
	}

	if len(mockDb.assertedProfiles) == 0 {
		t.Error("GetProfileByID was not called")
	}
}
