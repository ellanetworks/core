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
// - HomeNetworkKey.PrivateKey
func (db *Database) ExportSupportData(ctx context.Context) (map[string]any, error) {
	ctx, span := tracer.Start(ctx, "db/export_support_data")
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

		supportedTACs, _ := op.GetSupportedTacs()
		operatorMap := map[string]any{
			"ID":            op.ID,
			"Mcc":           op.Mcc,
			"Mnc":           op.Mnc,
			"OperatorCode":  op.OperatorCode,
			"SupportedTACs": supportedTACs,
		}
		out["operator"] = operatorMap
	}

	slices, err := db.ListNetworkSlices(ctx)
	if err != nil {
		logger.DBLog.Warn("failed to list network slices for support export", zap.Error(err))
	} else {
		if sAny, err := toAnySlice(slices); err != nil {
			logger.DBLog.Warn("failed to convert network slices for support export", zap.Error(err))

			out["network_slices"] = slices
		} else {
			out["network_slices"] = sAny
		}
	}

	hnKeys, err := db.ListHomeNetworkKeys(ctx)
	if err != nil {
		logger.DBLog.Warn("failed to list home network keys for support export", zap.Error(err))
	} else {
		redacted := make([]map[string]any, 0, len(hnKeys))
		for _, k := range hnKeys {
			redacted = append(redacted, map[string]any{
				"ID":            k.ID,
				"KeyIdentifier": k.KeyIdentifier,
				"Scheme":        k.Scheme,
				"PrivateKey":    "*",
			})
		}

		out["home_network_keys"] = redacted
	}

	profiles, _, err := db.ListProfilesPage(ctx, 1, 1000)
	if err != nil {
		logger.DBLog.Warn("failed to list profiles for support export", zap.Error(err))
	} else {
		if pAny, err := toAnySlice(profiles); err != nil {
			logger.DBLog.Warn("failed to convert profiles for support export", zap.Error(err))

			out["profiles"] = profiles
		} else {
			out["profiles"] = pAny
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

	// Include a list of subscribers (IMSI + assigned IP if any). Do not include
	// secret fields such as PermanentKey or Opc.
	subscribersList := []any{}

	perPage := 1000

	page := 1
	for {
		subs, total, err := db.ListSubscribersPage(ctx, page, perPage)
		if err != nil {
			logger.DBLog.Warn("failed to list subscribers for support export", zap.Error(err))
			break
		}

		for _, s := range subs {
			entry := map[string]any{"imsi": s.Imsi}
			subscribersList = append(subscribersList, entry)
		}

		// If we've collected all subscribers, stop paging.
		if len(subs) == 0 || page*perPage >= total {
			break
		}

		page++
	}

	out["subscribers"] = subscribersList

	// Include all IP leases (active and static reservations) so support
	// bundles show the full picture of address allocation.
	allLeases, err := db.listAllLeases(ctx)
	if err != nil {
		logger.DBLog.Warn("failed to list leases for support export", zap.Error(err))
	} else {
		leaseEntries := make([]any, 0, len(allLeases))
		for _, l := range allLeases {
			entry := map[string]any{
				"poolID":  l.PoolID,
				"address": l.Address,
				"imsi":    l.IMSI,
				"type":    l.Type,
			}
			if l.SessionID != nil {
				entry["sessionID"] = *l.SessionID
			}

			leaseEntries = append(leaseEntries, entry)
		}

		out["ip_leases"] = leaseEntries
	}

	// Include the last 100 radio logs (if any). Use the existing DB helper to
	// fetch the most recent entries; convert to a JSON-friendly []any using
	// toAnySlice so the support bundle contains structured entries.
	if logs, _, err := db.ListRadioEvents(ctx, 1, 100, nil); err != nil {
		logger.DBLog.Warn("failed to list radio logs for support export", zap.Error(err))
	} else {
		if lAny, err := toAnySlice(logs); err != nil {
			logger.DBLog.Warn("failed to convert radio logs for support export", zap.Error(err))

			out["radio_logs"] = logs
		} else {
			out["radio_logs"] = lAny
		}
	}

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
