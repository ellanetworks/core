package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ellanetworks/core/fleet"
	"github.com/ellanetworks/core/fleet/client"
	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
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

func RegisterFleet(dbInstance *db.Database, cfg config.Config) http.HandlerFunc {
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

		err := register(r.Context(), dbInstance, FleetURL, params.ActivationToken, cfg)
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

func register(ctx context.Context, dbInstance *db.Database, fleetURL string, activationToken string, cfg config.Config) error {
	key, err := dbInstance.LoadOrGenerateFleetKey(ctx)
	if err != nil {
		return fmt.Errorf("couldn't load or generate key: %w", err)
	}

	fC := client.New(fleetURL)

	initialConfig, err := buildInitialConfig(ctx, dbInstance, cfg)
	if err != nil {
		return fmt.Errorf("couldn't build initial config: %w", err)
	}

	data, err := fC.Register(ctx, activationToken, key.PublicKey, initialConfig)
	if err != nil {
		return fmt.Errorf("couldn't register to fleet: %w", err)
	}

	logger.EllaLog.Info("Registered to fleet successfully")

	err = dbInstance.UpdateFleetCredentials(ctx, []byte(data.Certificate), []byte(data.CACertificate))
	if err != nil {
		return fmt.Errorf("couldn't store fleet credentials in database: %w", err)
	}

	logger.EllaLog.Info("Fleet credentials stored successfully")

	err = fleet.ResumeSync(ctx, fleetURL, key, []byte(data.Certificate), []byte(data.CACertificate))
	if err != nil {
		return fmt.Errorf("couldn't start fleet sync: %w", err)
	}

	return nil
}

func buildInitialConfig(ctx context.Context, dbInstance *db.Database, cfg config.Config) (client.EllaCoreConfig, error) {
	op, err := dbInstance.GetOperator(ctx)
	if err != nil {
		return client.EllaCoreConfig{}, fmt.Errorf("couldn't get operator from database: %w", err)
	}

	supportedTacs, err := op.GetSupportedTacs()
	if err != nil {
		return client.EllaCoreConfig{}, fmt.Errorf("couldn't get supported tacs: %w", err)
	}

	routes, _, err := dbInstance.ListRoutesPage(ctx, 1, 100)
	if err != nil {
		return client.EllaCoreConfig{}, fmt.Errorf("couldn't list routes: %w", err)
	}

	routesConfigs := make([]client.Route, len(routes))
	for i, r := range routes {
		routesConfigs[i] = client.Route{
			ID:          r.ID,
			Destination: r.Destination,
			Gateway:     r.Gateway,
			Interface:   r.Interface.String(),
			Metric:      r.Metric,
		}
	}

	natEnabled, err := dbInstance.IsNATEnabled(ctx)
	if err != nil {
		return client.EllaCoreConfig{}, fmt.Errorf("couldn't get NAT configuration: %w", err)
	}

	n3Settings, err := dbInstance.GetN3Settings(ctx)
	if err != nil {
		return client.EllaCoreConfig{}, fmt.Errorf("couldn't get N3 settings: %w", err)
	}

	networkInterfacesConfigs := client.NetworkInterfaces{
		N2: client.N2Interface{
			Address: cfg.Interfaces.N2.Address,
			Port:    cfg.Interfaces.N2.Port,
		},
		N3: client.N3Interface{
			Name:            cfg.Interfaces.N3.Name,
			Address:         cfg.Interfaces.N3.Address,
			ExternalAddress: n3Settings.ExternalAddress,
		},
		N6: client.N6Interface{
			Name: cfg.Interfaces.N6.Name,
		},
		API: client.APIInterface{
			Address: cfg.Interfaces.API.Address,
			Port:    cfg.Interfaces.API.Port,
		},
	}

	if cfg.Interfaces.N3.VlanConfig != nil {
		networkInterfacesConfigs.N3.Vlan = &client.Vlan{
			MasterInterface: cfg.Interfaces.N3.VlanConfig.MasterInterface,
			VlanId:          cfg.Interfaces.N3.VlanConfig.VlanId,
		}
	}

	if cfg.Interfaces.N6.VlanConfig != nil {
		networkInterfacesConfigs.N6.Vlan = &client.Vlan{
			MasterInterface: cfg.Interfaces.N6.VlanConfig.MasterInterface,
			VlanId:          cfg.Interfaces.N6.VlanConfig.VlanId,
		}
	}

	dataNetworks, _, err := dbInstance.ListDataNetworksPage(ctx, 1, 100)
	if err != nil {
		return client.EllaCoreConfig{}, fmt.Errorf("couldn't list data networks: %w", err)
	}

	dnConfigs := make([]client.DataNetwork, len(dataNetworks))
	for i, dn := range dataNetworks {
		dnConfigs[i] = client.DataNetwork{
			ID:     dn.ID,
			Name:   dn.Name,
			IPPool: dn.IPPool,
			DNS:    dn.DNS,
			MTU:    dn.MTU,
		}
	}

	policies, _, err := dbInstance.ListPoliciesPage(ctx, 1, 100)
	if err != nil {
		return client.EllaCoreConfig{}, fmt.Errorf("couldn't list policies: %w", err)
	}

	policyConfigs := make([]client.Policy, len(policies))
	for i, p := range policies {
		policyConfigs[i] = client.Policy{
			ID:              p.ID,
			Name:            p.Name,
			BitrateUplink:   p.BitrateUplink,
			BitrateDownlink: p.BitrateDownlink,
			Var5qi:          p.Var5qi,
			Arp:             p.Arp,
			DataNetworkID:   p.DataNetworkID,
		}
	}

	subscribers, _, err := dbInstance.ListSubscribersPage(ctx, 1, 1000)
	if err != nil {
		return client.EllaCoreConfig{}, fmt.Errorf("couldn't list subscribers: %w", err)
	}

	subscriberConfigs := make([]client.Subscriber, len(subscribers))
	for i, s := range subscribers {
		subscriberConfigs[i] = client.Subscriber{
			ID:             s.ID,
			Imsi:           s.Imsi,
			IPAddress:      s.IPAddress,
			SequenceNumber: s.SequenceNumber,
			PermanentKey:   s.PermanentKey,
			Opc:            s.Opc,
			PolicyID:       s.PolicyID,
		}
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
		Networking: client.Networking{
			DataNetworks:      dnConfigs,
			Routes:            routesConfigs,
			NAT:               natEnabled,
			NetworkInterfaces: networkInterfacesConfigs,
		},
		Policies:    policyConfigs,
		Subscribers: subscriberConfigs,
	}

	return initialConfig, nil
}
