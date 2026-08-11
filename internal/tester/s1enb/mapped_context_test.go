// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb_test

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/internal/epskeys"
	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
)

func testKAMF() []byte {
	k := make([]byte, 32)
	for i := range k {
		k[i] = byte(i * 7)
	}

	return k
}

func TestMappedSecurityContextAgreesWithTheCore(t *testing.T) {
	const (
		dlCount = 11
		ulCount = 7
	)

	mapped, err := interworking.MapToEPSOnHandover(interworking.FiveGToEPSInput{
		KAMF:       testKAMF(),
		NgKSI:      nas.KeySetIdentifier{Value: 4},
		ULNASCount: ulCount,
		DLNASCount: dlCount,
		Algorithms: interworking.EPSNASAlgorithms{
			Ciphering: nas.CipheringAES,
			Integrity: nas.IntegrityAES,
		},
		UESecurityCapability: eps.UESecurityCapability{EEA: 0xf0, EIA: 0x70},
	})
	if err != nil {
		t.Fatalf("MapToEPSOnHandover: %v", err)
	}

	e := s1enb.NewUnboundUE()

	if err := e.InstallMappedSecurityContext(s1enb.MappedFrom5GS{
		KAMF:             testKAMF(),
		DownlinkNASCount: dlCount,
		UplinkNASCount:   ulCount,
		Ciphering:        uint8(nas.CipheringAES),
		Integrity:        uint8(nas.IntegrityAES),
		EKSI:             4,
	}); err != nil {
		t.Fatalf("InstallMappedSecurityContext: %v", err)
	}

	if !bytes.Equal(e.MappedKASME(), mapped.Context.KASME[:]) {
		t.Fatalf("K'ASME = % x, want the % x the AMF derived", e.MappedKASME(), mapped.Context.KASME)
	}

	// TS 33.401 Annex A.7
	wantEnc, err := epskeys.DeriveKNASEnc(mapped.Context.KASME[:], nas.CipheringAES)
	if err != nil {
		t.Fatalf("DeriveKNASEnc: %v", err)
	}

	wantInt, err := epskeys.DeriveKNASInt(mapped.Context.KASME[:], nas.IntegrityAES)
	if err != nil {
		t.Fatalf("DeriveKNASInt: %v", err)
	}

	knasEnc, knasInt := e.MappedNASKeys()

	if !bytes.Equal(knasEnc[:], wantEnc[:]) {
		t.Errorf("K_NASenc = % x, want % x", knasEnc, wantEnc)
	}

	if !bytes.Equal(knasInt[:], wantInt[:]) {
		t.Errorf("K_NASint = % x, want % x", knasInt, wantInt)
	}
}

// TS 33.501 §8.3.2 step 8
func TestEstimateDownlinkNASCount(t *testing.T) {
	for _, tc := range []struct {
		name   string
		stored nas.Count
		sqn    uint8
		want   uint32
		wantOK bool
	}{
		{"next in the same overflow window", nas.MakeCount(0, 10), 11, 11, true},
		{"wraps into the next window", nas.MakeCount(0, 250), 3, 0x0103, true},
		{"equal to the stored sequence number wraps", nas.MakeCount(0, 10), 10, 0x010a, true},
		{"below the stored sequence number wraps", nas.MakeCount(0, 200), 5, 0x0105, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := s1enb.EstimateDownlinkNASCount(tc.stored, tc.sqn)
			if (err == nil) != tc.wantOK {
				t.Fatalf("err = %v, want ok = %t", err, tc.wantOK)
			}

			if tc.wantOK && got.Value() != tc.want {
				t.Errorf("estimate = %d, want %d", got.Value(), tc.want)
			}
		})
	}
}

func TestContainerSequenceNumberRebuildsTheCount(t *testing.T) {
	const dlCount = nas.Count(0x0102fe)

	container := fgs.NewN1ModeToS1ModeNASTransparentContainer(dlCount)

	got, err := s1enb.EstimateDownlinkNASCount(nas.MakeCount(0x0102, 0xfd), container.SequenceNumber)
	if err != nil {
		t.Fatalf("EstimateDownlinkNASCount: %v", err)
	}

	if got != dlCount {
		t.Errorf("rebuilt count = %#06x, want %#06x", got.Value(), dlCount.Value())
	}
}
