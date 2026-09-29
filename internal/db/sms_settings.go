// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/netip"

	"github.com/prometheus/client_golang/prometheus"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

const SMSSettingsTableName = "sms_settings"

const smsSchema = 22

const DefaultSMSCPort = 3868

const maxMSISDNDigits = 15

const (
	getSMSSettingsStmt    = "SELECT &SMSSettings.* FROM %s WHERE singleton=TRUE"
	upsertSMSSettingsStmt = "INSERT INTO %s (singleton, smscAddress, smscPort, smsNumber) VALUES (TRUE, $SMSSettings.smscAddress, $SMSSettings.smscPort, $SMSSettings.smsNumber) ON CONFLICT(singleton) DO UPDATE SET smscAddress=excluded.smscAddress, smscPort=excluded.smscPort, smsNumber=excluded.smsNumber"
)

type SMSSettings struct {
	SMSCAddress string `db:"smscAddress"`
	SMSCPort    int    `db:"smscPort"`
	SMSNumber   string `db:"smsNumber"`
}

func DefaultSMSSettings() SMSSettings {
	return SMSSettings{SMSCPort: DefaultSMSCPort}
}

func (s SMSSettings) Enabled() bool {
	return s.SMSCAddress != ""
}

func (s SMSSettings) Validate() error {
	if s.SMSCPort < 1 || s.SMSCPort > 65535 {
		return fmt.Errorf("SMSC port must be between 1 and 65535, got %d", s.SMSCPort)
	}

	if s.SMSNumber != "" && !IsValidMSISDN(s.SMSNumber) {
		return fmt.Errorf("SMS number must be 1 to %d digits in E.164 international format, got %q", maxMSISDNDigits, s.SMSNumber)
	}

	if s.SMSCAddress == "" {
		return nil
	}

	addr, err := netip.ParseAddr(s.SMSCAddress)
	if err != nil || addr.Zone() != "" || addr.IsUnspecified() {
		return fmt.Errorf("SMSC address must be an IPv4 or IPv6 address, got %q", s.SMSCAddress)
	}

	if s.SMSNumber == "" {
		return errors.New("SMS number is required when an SMSC is set")
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

func (db *Database) GetSMSSettings(ctx context.Context) (*SMSSettings, error) {
	ctx, span := startSMSSettingsSpan(ctx, "SELECT")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SMSSettingsTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SMSSettingsTableName, "select").Inc()

	settings := DefaultSMSSettings()

	if !db.appliedSchemaAtLeast(ctx, smsSchema) {
		return &settings, nil
	}

	err := db.conn().Query(ctx, db.getSMSSettingsStmt).Get(&settings)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			settings = DefaultSMSSettings()

			return &settings, nil
		}

		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &settings, nil
}

func (db *Database) UpdateSMSSettings(ctx context.Context, settings *SMSSettings) error {
	if err := settings.Validate(); err != nil {
		return err
	}

	ctx, span := startSMSSettingsSpan(ctx, "UPSERT")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SMSSettingsTableName, "upsert"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SMSSettingsTableName, "upsert").Inc()

	if _, err := opUpdateSMSSettings.Invoke(ctx, db, settings); err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

func (db *Database) applyUpdateSMSSettings(ctx context.Context, s *SMSSettings) (any, error) {
	if err := db.runner(ctx).Query(ctx, db.upsertSMSSettingsStmt, s).Run(); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func startSMSSettingsSpan(ctx context.Context, operation string) (context.Context, trace.Span) {
	querySummary := fmt.Sprintf("%s %s", operation, SMSSettingsTableName)

	return tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName(operation),
			semconv.DBCollectionName(SMSSettingsTableName),
		),
	)
}
