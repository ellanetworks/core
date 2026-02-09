package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ellanetworks/core/fleet/client"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/version"
	"go.uber.org/zap"
)

type RegisterFleetParams struct {
	ActivationToken string `json:"activationToken"`
}

const (
	RegisterFleetAction = "register_fleet"
)

const (
	FleetURL = "https://127.0.0.1:5003"
)

func RegisterFleet(dbInstance *db.Database) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		email := r.Context().Value(contextKeyEmail)

		emailStr, ok := email.(string)
		if !ok {
			writeError(w, http.StatusInternalServerError, "Failed to get email", nil, logger.APILog)
			return
		}

		var params RegisterFleetParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		if params.ActivationToken == "" {
			writeError(w, http.StatusBadRequest, "activationToken is missing", nil, logger.APILog)
			return
		}

		err := register(r.Context(), dbInstance, FleetURL, params.ActivationToken)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to register to fleet", err, logger.APILog)
			return
		}

		logger.LogAuditEvent(
			r.Context(),
			RegisterFleetAction,
			emailStr,
			getClientIP(r),
			"User registered Core to Fleet",
		)

		writeResponse(w, SuccessResponse{Message: "Core registered to Fleet successfully"}, http.StatusCreated, logger.APILog)
	}
}

func register(ctx context.Context, dbInstance *db.Database, fleetURL string, activationToken string) error {
	key, err := dbInstance.LoadOrGenerateFleetKey(ctx)
	if err != nil {
		return fmt.Errorf("couldn't load or generate key: %w", err)
	}

	fC := client.New(fleetURL)

	initialConfig, err := buildInitialConfig(ctx, dbInstance)
	if err != nil {
		return fmt.Errorf("couldn't build initial config: %w", err)
	}

	data, err := fC.Register(ctx, activationToken, key.PublicKey, initialConfig)
	if err != nil {
		return fmt.Errorf("couldn't register to fleet: %w", err)
	}

	err = dbInstance.UpdateFleetCredentials(ctx, []byte(data.Certificate), []byte(data.CACertificate))
	if err != nil {
		return fmt.Errorf("couldn't store fleet credentials in database: %w", err)
	}

	err = fC.ConfigureMTLS(data.Certificate, key, data.CACertificate)
	if err != nil {
		return fmt.Errorf("couldn't configure mTLS: %w", err)
	}

	ticker := time.NewTicker(5 * time.Second)

	syncParams := &client.SyncParams{
		Version: version.GetVersion().Version,
	}

	go func() {
		for {
			select {
			case <-ticker.C:
				if err := fC.Sync(ctx, syncParams); err != nil {
					logger.EllaLog.Error("sync failed", zap.Error(err))
				}
			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()

	if err := fC.Sync(ctx, syncParams); err != nil {
		return fmt.Errorf("initial sync failed: %w", err)
	}

	logger.EllaLog.Info("Sync sent successfully")

	return nil
}

func buildInitialConfig(ctx context.Context, dbInstance *db.Database) (client.EllaCoreConfig, error) {
	op, err := dbInstance.GetOperator(ctx)
	if err != nil {
		return client.EllaCoreConfig{}, fmt.Errorf("couldn't get operator from database: %w", err)
	}

	supportedTacs, err := op.GetSupportedTacs()
	if err != nil {
		return client.EllaCoreConfig{}, fmt.Errorf("couldn't get supported tacs: %w", err)
	}

	initialConfig := client.EllaCoreConfig{
		Operator: client.Operator{
			ID: client.OperatorID{
				Mcc: op.Mcc,
				Mnc: op.Mnc,
			},
			Slice: client.OperatorSlice{
				Sst: op.Sst,
				Sd:  op.Sd,
			},
			OperatorCode: op.OperatorCode,
			Tracking: client.OperatorTracking{
				SupportedTacs: supportedTacs,
			},
			HomeNetwork: client.OperatorHomeNetwork{
				PrivateKey: op.HomeNetworkPrivateKey,
			},
		},
	}

	return initialConfig, nil
}
