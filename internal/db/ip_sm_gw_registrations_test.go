// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

func TestIPSMGWRegistration(t *testing.T) {
	dbInstance := setupTestDB(t)
	ctx := context.Background()

	const imsi = "001010000000061"

	createIMSSubscriber(t, dbInstance, imsi)

	if _, err := dbInstance.GetIPSMGWRegistration(ctx, imsi); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("initial registration: %v, want not found", err)
	}

	reg := db.IPSMGWRegistration{
		IMSI: imsi, Number: "15550000100", DiameterHost: "ims.ims.mnc001.mcc001.3gppnetwork.org",
		DiameterRealm: "ims.mnc001.mcc001.3gppnetwork.org",
	}

	if err := dbInstance.PutIPSMGWRegistration(ctx, reg); err != nil {
		t.Fatal(err)
	}

	reg.Number = "15550000101"

	if err := dbInstance.PutIPSMGWRegistration(ctx, reg); err != nil {
		t.Fatal(err)
	}

	if got, err := dbInstance.GetIPSMGWRegistration(ctx, imsi); err != nil || !reflect.DeepEqual(*got, reg) {
		t.Fatalf("registration = %+v, %v, want %+v", got, err, reg)
	}

	if all, err := dbInstance.ListAllIPSMGWRegistrations(ctx); err != nil || len(all) != 1 {
		t.Fatalf("all = %+v, %v", all, err)
	}

	if err := dbInstance.PutIPSMGWRegistration(ctx, db.IPSMGWRegistration{
		IMSI: "001010000000099", Number: "1", DiameterHost: "h", DiameterRealm: "r",
	}); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("registration of no subscriber: %v, want not found", err)
	}

	if err := dbInstance.DeleteIPSMGWRegistration(ctx, imsi); err != nil {
		t.Fatal(err)
	}

	if _, err := dbInstance.GetIPSMGWRegistration(ctx, imsi); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("deleted registration: %v, want not found", err)
	}

	if err := dbInstance.PutIPSMGWRegistration(ctx, reg); err != nil {
		t.Fatal(err)
	}

	if err := dbInstance.DeleteSubscriber(ctx, imsi); err != nil {
		t.Fatal(err)
	}

	if all, _ := dbInstance.ListAllIPSMGWRegistrations(ctx); len(all) != 0 {
		t.Fatalf("registrations of a deleted subscriber = %+v", all)
	}
}
