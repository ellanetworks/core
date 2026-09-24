// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"sync"
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

func TestPathSwitchRepointsTheConnectionWhileItIsRead(t *testing.T) {
	a := New(nil, nil, nil)
	source := &Radio{amf: a, name: "gnb-1", Conn: &downlinkOrderConn{}}
	target := &Radio{amf: a, name: "gnb-2", Conn: &downlinkOrderConn{}}
	ue := NewUeContext()

	ueConn, err := a.NewUeConn(source, models.RanUeNgapID(1))
	if err != nil {
		t.Fatalf("NewUeConn: %v", err)
	}

	var wg sync.WaitGroup

	stop := make(chan struct{})

	wg.Go(func() {
		defer close(stop)

		for i := range 5000 {
			radio := source
			if i%2 == 0 {
				radio = target
			}

			a.CommitPathSwitch(t.Context(), ue, ueConn, radio, models.RanUeNgapID(i), [32]uint8{}, 0)
		}
	})

	wg.Go(func() {
		for {
			select {
			case <-stop:
				return
			default:
			}

			if _, conn, err := ueConn.sendTarget(); err != nil || (conn != source.Conn && conn != target.Conn) {
				t.Errorf("sendTarget() = %v, %v, want one of the two radios", conn, err)

				return
			}
		}
	})

	wg.Wait()
}

func TestRegistrationFieldsAreWrittenWhileTheUEIsExported(t *testing.T) {
	a := New(nil, nil, nil)
	ue := NewUeContext()
	guami := &models.Guami{PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}, AmfID: "cafe00"}
	tai := models.Tai{PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}, Tac: "000001"}

	imei, err := etsi.NewIMEIFromPEI("imei-490154203237518")
	if err != nil {
		t.Fatalf("NewIMEIFromPEI: %v", err)
	}

	suci := &fgs.SUCI{Format: fgs.SUPIFormatIMSI, PLMN: nas.PLMN{MCC: "001", MNC: "01"}}

	var wg sync.WaitGroup

	stop := make(chan struct{})

	wg.Go(func() {
		defer close(stop)

		for range 5000 {
			ue.SetSUCI(suci)
			ue.SetImei(imei)
			ue.SetAllowedNssai([]models.Snssai{{Sst: 1}})
			ue.AllocateRegistrationArea([]models.Tai{tai})
			ue.SetRadioCapability([]byte{0x01})
			ue.SetAmbr(&models.Ambr{})
			ue.SetLocation(models.UserLocation{}, tai)
		}
	})

	wg.Go(func() {
		for {
			select {
			case <-stop:
				return
			default:
			}

			a.exportUeContext(guami, ue)
			ue.Snapshot()
		}
	})

	wg.Wait()
}
