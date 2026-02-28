package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/ellanetworks/core/fleet"
	"github.com/ellanetworks/core/fleet/client"
	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

type RegisterFleetParams struct {
	ActivationToken string `json:"activationToken"`
}

type UpdateFleetURLParams struct {
	URL string `json:"url"`
}

type FleetURLResponse struct {
	URL string `json:"url"`
}

const (
	RegisterFleetAction   = "register_fleet"
	UnregisterFleetAction = "unregister_fleet"
	UpdateFleetURLAction  = "update_fleet_url"
)

func RegisterFleet(dbInstance *db.Database, cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		email := r.Context().Value(contextKeyEmail)

		emailStr, ok := email.(string)
		if !ok {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get email", nil, logger.APILog)
			return
		}

		var params RegisterFleetParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		if params.ActivationToken == "" {
			writeError(r.Context(), w, http.StatusBadRequest, "activationToken is missing", nil, logger.APILog)
			return
		}

		err := register(r.Context(), dbInstance, params.ActivationToken, cfg)
		if err != nil {
			if errors.Is(err, client.ErrUnauthorized) {
				writeError(r.Context(), w, http.StatusUnauthorized, "Invalid activation code", err, logger.APILog)
				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to register to fleet", err, logger.APILog)

			return
		}

		logger.LogAuditEvent(
			r.Context(),
			RegisterFleetAction,
			emailStr,
			getClientIP(r),
			"User registered Core to Fleet",
		)

		writeResponse(r.Context(), w, SuccessResponse{Message: "Core registered to Fleet successfully"}, http.StatusCreated, logger.APILog)
	}
}

func register(ctx context.Context, dbInstance *db.Database, activationToken string, cfg config.Config) error {
	fleetURL, err := dbInstance.GetFleetURL(ctx)
	if err != nil {
		return fmt.Errorf("couldn't get fleet URL from database: %w", err)
	}

	if fleetURL == "" {
		return fmt.Errorf("fleet URL is not configured")
	}

	key, err := dbInstance.LoadOrGenerateFleetKey(ctx)
	if err != nil {
		return fmt.Errorf("couldn't load or generate key: %w", err)
	}

	fC := client.New(fleetURL)

	initialConfig, err := buildInitialConfig(ctx, dbInstance)
	if err != nil {
		return fmt.Errorf("couldn't build initial config: %w", err)
	}

	initialStatus := BuildStatus(ctx, dbInstance, cfg)
	initialMetrics := BuildMetrics()
	initialUsage := collectInitialUsage(ctx, dbInstance)

	data, err := fC.Register(ctx, activationToken, key.PublicKey, initialConfig, initialStatus, initialMetrics, initialUsage)
	if err != nil {
		return fmt.Errorf("couldn't register to fleet: %w", err)
	}

	logger.EllaLog.Info("Registered to fleet successfully")

	err = dbInstance.UpdateFleetCredentials(ctx, []byte(data.Certificate), []byte(data.CACertificate))
	if err != nil {
		return fmt.Errorf("couldn't store fleet credentials in database: %w", err)
	}

	logger.EllaLog.Info("Fleet credentials stored successfully")

	statusProvider := func() client.EllaCoreStatus {
		return BuildStatus(context.Background(), dbInstance, cfg)
	}

	metricsProvider := func() client.EllaCoreMetrics {
		return BuildMetrics()
	}

	err = fleet.ResumeSync(context.Background(), fleetURL, key, []byte(data.Certificate), []byte(data.CACertificate), dbInstance, statusProvider, metricsProvider, func(syncCtx context.Context, success bool) {
		if success {
			if err := dbInstance.UpdateFleetSyncStatus(syncCtx); err != nil {
				logger.EllaLog.Error("couldn't update fleet sync status", zap.Error(err))
			}
		}
	})
	if err != nil {
		return fmt.Errorf("couldn't start fleet sync: %w", err)
	}

	return nil
}

func BuildStatus(ctx context.Context, dbInstance *db.Database, cfg config.Config) client.EllaCoreStatus {
	networkInterfacesConfigs := client.StatusNetworkInterfaces{
		N2: client.N2Interface{
			Address: cfg.Interfaces.N2.Address,
			Port:    cfg.Interfaces.N2.Port,
		},
		N3: client.N3Interface{
			Name:    cfg.Interfaces.N3.Name,
			Address: cfg.Interfaces.N3.Address,
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

	radios := getRadiosStatus()
	subscribers := getSubscribersStatus(ctx, dbInstance)

	return client.EllaCoreStatus{
		NetworkInterfaces: networkInterfacesConfigs,
		Radios:            radios,
		Subscribers:       subscribers,
	}
}

func BuildMetrics() client.EllaCoreMetrics {
	metrics := client.EllaCoreMetrics{}

	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		logger.EllaLog.Warn("failed to gather prometheus metrics", zap.Error(err))
		return metrics
	}

	for _, mf := range mfs {
		name := mf.GetName()

		ms := mf.GetMetric()
		if len(ms) == 0 {
			continue
		}

		m := ms[0]

		switch name {
		case "app_uplink_bytes":
			if c := m.GetCounter(); c != nil {
				metrics.UplinkBytesTotal = int64(math.Round(c.GetValue()))
			}
		case "app_downlink_bytes":
			if c := m.GetCounter(); c != nil {
				metrics.DownlinkBytesTotal = int64(math.Round(c.GetValue()))
			}
		case "app_pdu_sessions_total":
			if g := m.GetGauge(); g != nil {
				metrics.PDUSessionsTotal = int64(math.Round(g.GetValue()))
			}
		case "process_cpu_seconds_total":
			if c := m.GetCounter(); c != nil {
				metrics.ProcessCPUSecondsTotal = c.GetValue()
			}
		case "process_resident_memory_bytes":
			if g := m.GetGauge(); g != nil {
				metrics.ProcessResidentMemoryBytes = int64(math.Round(g.GetValue()))
			}
		case "go_goroutines":
			if g := m.GetGauge(); g != nil {
				metrics.GoGoroutines = int64(math.Round(g.GetValue()))
			}
		case "process_start_time_seconds":
			if g := m.GetGauge(); g != nil {
				metrics.ProcessStartTimeSeconds = g.GetValue()
			}
		case "app_database_storage_bytes":
			if g := m.GetGauge(); g != nil {
				metrics.DatabaseStorageBytes = int64(math.Round(g.GetValue()))
			}
		case "app_ip_addresses_total":
			if g := m.GetGauge(); g != nil {
				metrics.IPAddresses = int64(math.Round(g.GetValue()))
			}
		case "app_ip_addresses_allocated_total":
			if g := m.GetGauge(); g != nil {
				metrics.IPAddressesAllocated = int64(math.Round(g.GetValue()))
			}
		case "app_registration_attempts_total":
			for _, sample := range ms {
				var result string

				for _, lp := range sample.GetLabel() {
					if lp.GetName() == "result" {
						result = lp.GetValue()
					}
				}

				if c := sample.GetCounter(); c != nil {
					switch result {
					case "accept":
						metrics.RegistrationAttemptsAccepted += int64(math.Round(c.GetValue()))
					case "reject":
						metrics.RegistrationAttemptsRejected += int64(math.Round(c.GetValue()))
					}
				}
			}
		}
	}

	return metrics
}

func getRadiosStatus() []client.Radio {
	amf := amfContext.AMFSelf()
	_, ranList := amf.ListAmfRan(1, 1000)

	radios := make([]client.Radio, 0, len(ranList))
	for _, radio := range ranList {
		supportedTAIs := make([]client.SupportedTAI, 0, len(radio.SupportedTAIs))
		for _, tai := range radio.SupportedTAIs {
			snssais := make([]client.Snssai, 0, len(tai.SNssaiList))
			for _, snssai := range tai.SNssaiList {
				snssais = append(snssais, client.Snssai{
					Sst: snssai.Sst,
					Sd:  snssai.Sd,
				})
			}

			supportedTAIs = append(supportedTAIs, client.SupportedTAI{
				Tai: client.Tai{
					PlmnID: client.PlmnID{
						Mcc: tai.Tai.PlmnID.Mcc,
						Mnc: tai.Tai.PlmnID.Mnc,
					},
					Tac: tai.Tai.Tac,
				},
				SNssais: snssais,
			})
		}

		radioAddress := ""

		if radio.Conn != nil {
			if addr := radio.Conn.RemoteAddr(); addr != nil {
				radioAddress = addr.String()
			}
		}

		radioID := ""
		if radio.RanID.GNbID != nil {
			radioID = radio.RanID.GNbID.GNBValue
		}

		radios = append(radios, client.Radio{
			Name:          radio.Name,
			ID:            radioID,
			Address:       radioAddress,
			SupportedTAIs: supportedTAIs,
		})
	}

	return radios
}

func getSubscribersStatus(ctx context.Context, dbInstance *db.Database) []client.SubscriberStatus {
	subscribers, _, err := dbInstance.ListSubscribersPage(ctx, 1, 1000)
	if err != nil {
		logger.EllaLog.Error("failed to list subscribers for status", zap.Error(err))
		return nil
	}

	amf := amfContext.AMFSelf()

	statuses := make([]client.SubscriberStatus, 0, len(subscribers))
	for _, s := range subscribers {
		ipAddress := ""
		if s.IPAddress != nil {
			ipAddress = *s.IPAddress
		}

		statuses = append(statuses, client.SubscriberStatus{
			Imsi:       s.Imsi,
			Registered: amf.IsSubscriberRegistered(s.Imsi),
			IPAddress:  ipAddress,
		})
	}

	return statuses
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
			N3ExternalAddress: n3Settings.ExternalAddress,
		},
		Policies:    policyConfigs,
		Subscribers: subscriberConfigs,
	}

	return initialConfig, nil
}

func UnregisterFleet(dbInstance *db.Database) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		email := r.Context().Value(contextKeyEmail)

		emailStr, ok := email.(string)
		if !ok {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get email", nil, logger.APILog)
			return
		}

		managed, err := dbInstance.IsFleetManaged(r.Context())
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to check fleet status", err, logger.APILog)
			return
		}

		if !managed {
			writeError(r.Context(), w, http.StatusBadRequest, "Core is not registered to a Fleet", nil, logger.APILog)
			return
		}

		// Notify the fleet server about the unregistration.
		fleetData, err := dbInstance.GetFleet(r.Context())
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to load fleet data", err, logger.APILog)
			return
		}

		key, err := dbInstance.LoadOrGenerateFleetKey(r.Context())
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to load fleet key", err, logger.APILog)
			return
		}

		fC := client.New(fleetData.URL)

		if err := fC.ConfigureMTLS(string(fleetData.Certificate), key, string(fleetData.CACertificate)); err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to configure mTLS for fleet", err, logger.APILog)
			return
		}

		if err := fC.Unregister(r.Context()); err != nil {
			logger.APILog.Warn("couldn't notify fleet server about unregistration", zap.Error(err))
		}

		fleet.StopSync()

		err = dbInstance.ClearFleetCredentials(r.Context())
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to unregister from Fleet", err, logger.APILog)
			return
		}

		logger.LogAuditEvent(
			r.Context(),
			UnregisterFleetAction,
			emailStr,
			getClientIP(r),
			"User unregistered Core from Fleet",
		)

		writeResponse(r.Context(), w, SuccessResponse{Message: "Core unregistered from Fleet successfully"}, http.StatusOK, logger.APILog)
	}
}

// collectInitialUsage gathers recent per-subscriber daily usage counters
// to include in the Fleet registration payload.
func collectInitialUsage(ctx context.Context, dbInstance *db.Database) []client.SubscriberUsageEntry {
	const recentDays = 7

	now := time.Now().UTC()
	start := now.AddDate(0, 0, -(recentDays - 1))

	rows, err := dbInstance.GetRawDailyUsage(ctx, start, now)
	if err != nil {
		logger.APILog.Warn("failed to collect initial usage for fleet registration", zap.Error(err))
		return nil
	}

	entries := make([]client.SubscriberUsageEntry, 0, len(rows))
	for _, r := range rows {
		entries = append(entries, client.SubscriberUsageEntry{
			EpochDay:      r.EpochDay,
			IMSI:          r.IMSI,
			UplinkBytes:   r.BytesUplink,
			DownlinkBytes: r.BytesDownlink,
		})
	}

	return entries
}

func GetFleetURL(dbInstance *db.Database) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		url, err := dbInstance.GetFleetURL(r.Context())
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get fleet URL", err, logger.APILog)
			return
		}

		writeResponse(r.Context(), w, FleetURLResponse{URL: url}, http.StatusOK, logger.APILog)
	}
}

func UpdateFleetURL(dbInstance *db.Database) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		email := r.Context().Value(contextKeyEmail)

		emailStr, ok := email.(string)
		if !ok {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get email", nil, logger.APILog)
			return
		}

		var params UpdateFleetURLParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		if params.URL == "" {
			writeError(r.Context(), w, http.StatusBadRequest, "url is missing", nil, logger.APILog)
			return
		}

		err := dbInstance.UpdateFleetURL(r.Context(), params.URL)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to update fleet URL", err, logger.APILog)
			return
		}

		logger.LogAuditEvent(
			r.Context(),
			UpdateFleetURLAction,
			emailStr,
			getClientIP(r),
			fmt.Sprintf("User updated fleet URL to %s", params.URL),
		)

		writeResponse(r.Context(), w, SuccessResponse{Message: "Fleet URL updated successfully"}, http.StatusOK, logger.APILog)
	}
}
