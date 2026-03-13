package ausf

import (
	"testing"
	"time"

	"github.com/ellanetworks/core/etsi"
)

func mustSUPI(t *testing.T, imsi string) etsi.SUPI {
	t.Helper()

	s, err := etsi.NewSUPIFromIMSI(imsi)
	if err != nil {
		t.Fatalf("invalid IMSI %q: %v", imsi, err)
	}

	return s
}

func setupTestAUSF(t *testing.T) *AUSF {
	t.Helper()

	a := NewAUSF(nil)
	ausf = a

	return a
}

func TestPoolEntryDeletedAfterSuccessfulConfirm(t *testing.T) {
	a := setupTestAUSF(t)
	suci := "suci-0-001-01-0000-0-0-0000000001"
	expectedSUPI := mustSUPI(t, "001010000000001")

	a.addUeAuthenticationContextToPool(suci, &UEAuthenticationContext{
		Supi:      expectedSUPI,
		XresStar:  "abcdef1234567890abcdef1234567890",
		Kseaf:     "kseaf-value",
		Rand:      "rand-value",
		CreatedAt: time.Now(),
	})

	if a.getUeAuthenticationContext(suci) == nil {
		t.Fatal("expected pool entry to exist before confirm")
	}

	supi, kseaf, err := Auth5gAkaComfirmRequestProcedure("abcdef1234567890abcdef1234567890", suci)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if supi != expectedSUPI {
		t.Fatalf("unexpected supi: %s", supi)
	}

	if kseaf != "kseaf-value" {
		t.Fatalf("unexpected kseaf: %s", kseaf)
	}

	if a.getUeAuthenticationContext(suci) != nil {
		t.Fatal("expected pool entry to be deleted after successful confirm")
	}
}

func TestPoolEntryDeletedAfterFailedConfirm(t *testing.T) {
	a := setupTestAUSF(t)
	suci := "suci-0-001-01-0000-0-0-0000000002"

	a.addUeAuthenticationContextToPool(suci, &UEAuthenticationContext{
		Supi:      mustSUPI(t, "001010000000002"),
		XresStar:  "abcdef1234567890abcdef1234567890",
		Kseaf:     "kseaf-value",
		Rand:      "rand-value",
		CreatedAt: time.Now(),
	})

	_, _, err := Auth5gAkaComfirmRequestProcedure("wrong-xres-star-value-00000000000", suci)
	if err == nil {
		t.Fatal("expected error for wrong RES*")
	}

	if a.getUeAuthenticationContext(suci) != nil {
		t.Fatal("expected pool entry to be deleted after failed confirm")
	}
}

func TestPoolEntryNotFoundReturnsError(t *testing.T) {
	setupTestAUSF(t)

	_, _, err := Auth5gAkaComfirmRequestProcedure("anything", "nonexistent-suci")
	if err == nil {
		t.Fatal("expected error for missing pool entry")
	}
}

func TestCleanupEvictsExpiredEntries(t *testing.T) {
	a := setupTestAUSF(t)

	a.addUeAuthenticationContextToPool("stale-suci", &UEAuthenticationContext{
		Supi:      mustSUPI(t, "001010000000003"),
		XresStar:  "xres",
		CreatedAt: time.Now().Add(-2 * authContextTTL),
	})

	a.addUeAuthenticationContextToPool("fresh-suci", &UEAuthenticationContext{
		Supi:      mustSUPI(t, "001010000000004"),
		XresStar:  "xres",
		CreatedAt: time.Now(),
	})

	a.evictExpiredContexts()

	if a.getUeAuthenticationContext("stale-suci") != nil {
		t.Fatal("expected stale entry to be evicted")
	}

	if a.getUeAuthenticationContext("fresh-suci") == nil {
		t.Fatal("expected fresh entry to still exist")
	}
}
