// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"sync"
	"testing"

	"github.com/ellanetworks/core/etsi"
)

func TestEPSSessionsOfReadsAccessUnderTheSessionLock(t *testing.T) {
	supi, err := etsi.NewSUPIFromIMSI("001010000000001")
	if err != nil {
		t.Fatal(err)
	}

	sc := &SMContext{Supi: supi, Access: Access4G, Ref: "ref-1"}
	s := &SMF{pool: map[string]*SMContext{sc.Ref: sc}, byKey: make(map[string]*SMContext)}

	var wg sync.WaitGroup

	wg.Add(2)

	go func() {
		defer wg.Done()

		for i := range 2000 {
			sc.Mutex.Lock()

			if i%2 == 0 {
				sc.Access = Access5G
			} else {
				sc.Access = Access4G
			}

			sc.Mutex.Unlock()
		}
	}()

	go func() {
		defer wg.Done()

		for range 2000 {
			s.epsSessionsOf(supi)
		}
	}()

	wg.Wait()
}
