package db

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/supportbundle"
	"go.uber.org/zap"
)

// ExportSupportData returns a map containing exported data for support bundles.
// Sensitive fields are redacted at the source immediately after database fetch:
// - Operator.OperatorCode
// - Operator.HomeNetworkPrivateKey
func (db *Database) ExportSupportData(ctx context.Context) (map[string]any, error) {
	ctx, span := tracer.Start(ctx, "ExportSupportData")
	defer span.End()

	start := time.Now()
	capturedAt := time.Now().UTC().Format(time.RFC3339)

	out := map[string]any{}
	out["bundle_metadata"] = map[string]any{"version": "1.0", "captured_at": capturedAt}

	op, err := db.GetOperator(ctx)
	if err != nil {
		logger.DBLog.Warn("failed to get operator for support export", zap.Error(err))
	} else {
		// Redact sensitive operator fields immediately after fetch
		op.OperatorCode = "*"
		op.HomeNetworkPrivateKey = "*"

		// Convert operator to JSON-friendly map; Sd is emitted as hex string
		// (3 bytes -> 6 hex chars) to avoid base64 encoding in JSON
		supportedTACs, _ := op.GetSupportedTacs()
		operatorMap := map[string]any{
			"ID":                    op.ID,
			"Mcc":                   op.Mcc,
			"Mnc":                   op.Mnc,
			"OperatorCode":          op.OperatorCode,
			"SupportedTACs":         supportedTACs,
			"Sst":                   op.Sst,
			"Sd":                    op.GetHexSd(),
			"HomeNetworkPrivateKey": op.HomeNetworkPrivateKey,
		}
		out["operator"] = operatorMap
	}

	policies, _, err := db.ListPoliciesPage(ctx, 1, 1000)
	if err != nil {
		logger.DBLog.Warn("failed to list policies for support export", zap.Error(err))
	} else {
		if pAny, err := toAnySlice(policies); err != nil {
			logger.DBLog.Warn("failed to convert policies for support export", zap.Error(err))

			out["policies"] = policies
		} else {
			out["policies"] = pAny
		}
	}

	dataNetworks, _, err := db.ListDataNetworksPage(ctx, 1, 1000)
	if err != nil {
		logger.DBLog.Warn("failed to list data networks for support export", zap.Error(err))
	} else {
		if nAny, err := toAnySlice(dataNetworks); err != nil {
			logger.DBLog.Warn("failed to convert data networks for support export", zap.Error(err))

			out["networking"] = dataNetworks
		} else {
			out["networking"] = nAny
		}
	}

	total, err := db.CountSubscribers(ctx)
	if err != nil {
		logger.DBLog.Warn("failed to count subscribers for support export", zap.Error(err))
	}

	withIP, err := db.CountSubscribersWithIP(ctx)
	if err != nil {
		logger.DBLog.Warn("failed to count subscribers with IP for support export", zap.Error(err))
	}

	out["subscribers_summary"] = map[string]any{"total": total, "with_ip": withIP}

	duration := time.Since(start)
	if DBQueryDuration != nil {
		DBQueryDuration.WithLabelValues("support_bundle", "export").Observe(duration.Seconds())
	}

	return out, nil
}

// toAnySlice marshals v to JSON and unmarshals into []any. It provides a
// JSON-friendly representation of typed slices (e.g. []Policy, []DataNetwork).
func toAnySlice(v any) ([]any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	var out []any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}

	return out, nil
}

// WriteSupportBundle streams a gzipped tar archive containing db.json to w.
// It gathers the redacted export via ExportSupportData and delegates to the
// supportbundle writer helper.
func (db *Database) WriteSupportBundle(ctx context.Context, w io.Writer) error {
	data, err := db.ExportSupportData(ctx)
	if err != nil {
		return err
	}

	return supportbundle.GenerateSupportBundleFromData(ctx, data, w)
}
