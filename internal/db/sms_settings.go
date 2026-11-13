// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

const SMSSettingsTableName = "sms_settings"

const smsSchema = 22

const DefaultSMSCPort = 3868

const maxMSISDNDigits = 15

const InitialSMSNumber = "15550001111"

const (
	getSMSSettingsSQL    = "SELECT smsNumber FROM " + SMSSettingsTableName + " WHERE singleton=TRUE"
	upsertSMSSettingsSQL = "INSERT INTO " + SMSSettingsTableName + " (singleton, smsNumber) VALUES (TRUE, ?) ON CONFLICT(singleton) DO UPDATE SET smsNumber=excluded.smsNumber"
)

type SMSSettings struct {
	SMSNumber string
}

func DefaultSMSSettings() SMSSettings {
	return SMSSettings{SMSNumber: InitialSMSNumber}
}

func (s SMSSettings) Validate() error {
	if s.SMSNumber != "" && !IsValidMSISDN(s.SMSNumber) {
		return fmt.Errorf("SMS number must be 1 to %d digits in E.164 international format, got %q", maxMSISDNDigits, s.SMSNumber)
	}

	return nil
}

func IsValidMSISDN(msisdn string) bool {
	if len(msisdn) == 0 || len(msisdn) > maxMSISDNDigits || msisdn[0] == '0' {
		return false
	}

	for i := range len(msisdn) {
		if msisdn[i] < '0' || msisdn[i] > '9' {
			return false
		}
	}

	return true
}

func (db *Database) GetSMSSettings(ctx context.Context) (_ *SMSSettings, err error) {
	ctx, finish := startDBQuery(ctx, SMSSettingsTableName, "SELECT")
	defer func() { finish(err) }()

	settings := DefaultSMSSettings()

	if !db.appliedSchemaAtLeast(ctx, smsSchema) {
		return &settings, nil
	}

	err = db.reader().QueryRowContext(ctx, getSMSSettingsSQL).Scan(&settings.SMSNumber)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			settings = DefaultSMSSettings()

			return &settings, nil
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &settings, nil
}

func (db *Database) UpdateSMSSettings(ctx context.Context, settings *SMSSettings) (err error) {
	if err := settings.Validate(); err != nil {
		return err
	}

	ctx, finish := startDBQuery(ctx, SMSSettingsTableName, "UPSERT")
	defer func() { finish(err) }()

	if _, err := opUpdateSMSSettings.Invoke(ctx, db, settings); err != nil {
		return err
	}

	return nil
}

func (db *Database) applyUpdateSMSSettings(ctx context.Context, s *SMSSettings) (any, error) {
	if _, err := db.runner(ctx).ExecContext(ctx, upsertSMSSettingsSQL, s.SMSNumber); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}
