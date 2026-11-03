// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/cluster/listener"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/roaming"
)

const DeregisterRoamerAction = "deregister_roamer"

type RoamerRegistration struct {
	RAT             string `json:"rat"`
	NodeID          string `json:"nodeID,omitempty"`
	Reachable       bool   `json:"reachable"`
	Registered      bool   `json:"registered"`
	ConnectionState string `json:"connectionState,omitempty"`
	Since           string `json:"since,omitempty"`
	LastSeenAt      string `json:"lastSeenAt,omitempty"`
	Radio           string `json:"radio,omitempty"`
}

type RoamerSession struct {
	RAT         string `json:"rat"`
	NodeID      string `json:"nodeID,omitempty"`
	ID          uint8  `json:"id"`
	DataNetwork string `json:"dataNetwork,omitempty"`
	IPv4Address string `json:"ipv4Address,omitempty"`
	IPv6Prefix  string `json:"ipv6Prefix,omitempty"`
}

type RoamerUsage struct {
	UplinkBytes   int64 `json:"uplinkBytes"`
	DownlinkBytes int64 `json:"downlinkBytes"`
	TotalBytes    int64 `json:"totalBytes"`
}

type Roamer struct {
	IMSI          string               `json:"imsi"`
	PartnerID     string               `json:"partnerID"`
	PartnerName   string               `json:"partnerName"`
	HomePLMN      RoamingPLMN          `json:"homePLMN"`
	Registrations []RoamerRegistration `json:"registrations"`
	Sessions      []RoamerSession      `json:"sessions"`
	UsageToday    *RoamerUsage         `json:"usageToday,omitempty"`
}

type UnreachableNode struct {
	NodeID string `json:"nodeID"`
	Error  string `json:"error"`
}

type ListRoamersResponse struct {
	Items            []Roamer          `json:"items"`
	UnreachableNodes []UnreachableNode `json:"unreachableNodes,omitempty"`
}

var errRoamerStillRegistered = errors.New("the roamer is still registered (the node that holds it may be unreachable); retry later")

var localRoamers atomic.Pointer[func() []roaming.LiveRoamer]

func SetLocalRoamers(f func() []roaming.LiveRoamer) {
	localRoamers.Store(&f)
}

func loadLocalRoamers() ([]roaming.LiveRoamer, error) {
	f := localRoamers.Load()
	if f == nil {
		return nil, errors.New("the roamer view is not ready")
	}

	return (*f)(), nil
}

func clusterRoamersEndpoint() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		roamers, err := loadLocalRoamers()
		if err != nil {
			writeError(r.Context(), w, http.StatusServiceUnavailable, err.Error(), nil, logger.APILog)
			return
		}

		body, err := json.Marshal(roamers)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to encode the roamers", err, logger.APILog)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	})
}

type nodeRoamers struct {
	nodeID  string
	roamers []roaming.LiveRoamer
	err     error
}

func gatherRoamers(ctx context.Context, dbInstance *db.Database, ln *listener.Listener) []nodeRoamers {
	nodes := seppNodes(ctx, dbInstance)
	out := make([]nodeRoamers, len(nodes))

	var wg sync.WaitGroup

	for i, n := range nodes {
		out[i].nodeID = n.nodeID

		if n.self {
			out[i].roamers, out[i].err = loadLocalRoamers()
			continue
		}

		wg.Go(func() {
			status, body, err := clusterNodeDo(ctx, ln, n.nodeID, n.address, http.MethodGet, roaming.ClusterRoamersPath, nil)

			switch {
			case err != nil:
				out[i].err = fmt.Errorf("unreachable: %w", err)
			case status != http.StatusOK:
				out[i].err = fmt.Errorf("the node answered %d", status)
			default:
				out[i].roamers, out[i].err = roaming.DecodeLiveRoamers(body)
			}
		})
	}

	wg.Wait()

	return out
}

type roamerView struct {
	partners map[string]db.RoamingPartner
	regs     map[string][]db.UERegistration
	nodes    map[string]nodeRoamers
	usage    map[string]RoamerUsage
	withUse  bool
}

func ratOfRegistration(regType string) string {
	if regType == db.UERegistrationTypeMME {
		return roaming.RAT4G
	}

	return roaming.RAT5G
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}

	return t.UTC().Format(time.RFC3339)
}

func (v *roamerView) roamer(visitor db.Subscriber) Roamer {
	out := Roamer{
		IMSI:          visitor.Imsi,
		Registrations: []RoamerRegistration{},
		Sessions:      []RoamerSession{},
	}

	if visitor.RoamingPartnerID != nil {
		out.PartnerID = *visitor.RoamingPartnerID

		if p, ok := v.partners[out.PartnerID]; ok {
			out.PartnerName = p.Name

			if plmn, ok := p.PLMNOfIMSI(visitor.Imsi); ok {
				out.HomePLMN = RoamingPLMN{Mcc: plmn.Mcc, Mnc: plmn.Mnc}
			}
		}
	}

	for _, reg := range v.regs[visitor.Imsi] {
		rat := ratOfRegistration(reg.Type)
		entry := RoamerRegistration{RAT: rat, NodeID: reg.NodeID}

		node, known := v.nodes[reg.NodeID]
		if !known || node.err != nil {
			out.Registrations = append(out.Registrations, entry)
			continue
		}

		entry.Reachable = true

		i := slices.IndexFunc(node.roamers, func(r roaming.LiveRoamer) bool { return r.IMSI == visitor.Imsi && r.RAT == rat })
		if i < 0 {
			out.Registrations = append(out.Registrations, entry)
			continue
		}

		live := node.roamers[i]
		entry.Registered = live.Registered
		entry.ConnectionState = connectionState(true, live.Connected)
		entry.Since = formatTime(live.Since)
		entry.LastSeenAt = formatTime(live.LastSeenAt)
		entry.Radio = live.Radio

		if out.HomePLMN.Mcc == "" {
			out.HomePLMN = RoamingPLMN{Mcc: live.HomeMcc, Mnc: live.HomeMnc}
		}

		out.Registrations = append(out.Registrations, entry)

		for _, s := range live.Sessions {
			out.Sessions = append(out.Sessions, RoamerSession{
				RAT:         rat,
				NodeID:      reg.NodeID,
				ID:          s.ID,
				DataNetwork: s.DataNetwork,
				IPv4Address: s.IPv4Address,
				IPv6Prefix:  s.IPv6Prefix,
			})
		}
	}

	if v.withUse {
		u := v.usage[visitor.Imsi]
		out.UsageToday = &u
	}

	return out
}

func canReadRoamerUsage(r *http.Request) bool {
	roleID, ok := r.Context().Value(contextKeyRoleID).(RoleID)

	return ok && roleHasPermission(roleID, PermGetSubscriberUsage)
}

func buildRoamerView(r *http.Request, dbInstance *db.Database, ln *listener.Listener) (*roamerView, error) {
	ctx := r.Context()

	partners, err := dbInstance.ListRoamingPartners(ctx)
	if err != nil {
		return nil, err
	}

	regs, err := dbInstance.ListVisitorRegistrations(ctx)
	if err != nil {
		return nil, err
	}

	v := &roamerView{
		partners: make(map[string]db.RoamingPartner, len(partners)),
		regs:     map[string][]db.UERegistration{},
		nodes:    map[string]nodeRoamers{},
		withUse:  canReadRoamerUsage(r),
	}

	for _, p := range partners {
		v.partners[p.ID] = p
	}

	for _, reg := range regs {
		v.regs[reg.Imsi] = append(v.regs[reg.Imsi], reg)
	}

	for _, n := range gatherRoamers(ctx, dbInstance, ln) {
		v.nodes[n.nodeID] = n
	}

	if v.withUse {
		rows, err := dbInstance.ListRoamingUsageOfDay(ctx, db.DaysSinceEpoch(time.Now()))
		if err != nil {
			return nil, err
		}

		v.usage = make(map[string]RoamerUsage, len(rows))

		for _, row := range rows {
			v.usage[row.IMSI] = RoamerUsage{UplinkBytes: row.BytesUplink, DownlinkBytes: row.BytesDownlink, TotalBytes: row.BytesUplink + row.BytesDownlink}
		}
	}

	return v, nil
}

func (v *roamerView) unreachable() []UnreachableNode {
	var out []UnreachableNode

	for _, n := range v.nodes {
		if n.err != nil {
			out = append(out, UnreachableNode{NodeID: n.nodeID, Error: n.err.Error()})
		}
	}

	slices.SortFunc(out, func(a, b UnreachableNode) int { return strings.Compare(a.NodeID, b.NodeID) })

	return out
}

func ListRoamers(dbInstance *db.Database, ln *listener.Listener) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		visitors, err := dbInstance.ListVisitors(r.Context())
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to list roamers", err, logger.APILog)
			return
		}

		view, err := buildRoamerView(r, dbInstance, ln)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to list roamers", err, logger.APILog)
			return
		}

		resp := ListRoamersResponse{Items: make([]Roamer, 0, len(visitors)), UnreachableNodes: view.unreachable()}

		for _, visitor := range visitors {
			resp.Items = append(resp.Items, view.roamer(visitor))
		}

		writeResponse(r.Context(), w, resp, http.StatusOK, logger.APILog)
	})
}

func getVisitor(w http.ResponseWriter, r *http.Request, dbInstance *db.Database) (*db.Subscriber, bool) {
	imsi := r.PathValue("imsi")
	if _, err := etsi.NewSUPIFromIMSI(imsi); err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "invalid imsi: must be a valid IMSI of 6 to 15 digits", nil, logger.APILog)
		return nil, false
	}

	visitor, err := dbInstance.GetSubscriberOrVisitor(r.Context(), imsi)

	switch {
	case errors.Is(err, db.ErrNotFound) || (err == nil && visitor.RoamingPartnerID == nil):
		writeError(r.Context(), w, http.StatusNotFound, "Roamer not found", nil, logger.APILog)
		return nil, false
	case err != nil:
		writeError(r.Context(), w, http.StatusInternalServerError, "Failed to retrieve the roamer", err, logger.APILog)
		return nil, false
	}

	return visitor, true
}

func GetRoamer(dbInstance *db.Database, ln *listener.Listener) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		visitor, ok := getVisitor(w, r, dbInstance)
		if !ok {
			return
		}

		view, err := buildRoamerView(r, dbInstance, ln)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to retrieve the roamer", err, logger.APILog)
			return
		}

		writeResponse(r.Context(), w, view.roamer(*visitor), http.StatusOK, logger.APILog)
	})
}

func DeregisterRoamer(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)

		visitor, ok := getVisitor(w, r, dbInstance)
		if !ok {
			return
		}

		imsi := visitor.Imsi

		err := deregisterRoamer(r.Context(), dbInstance, imsi)

		switch {
		case errors.Is(err, db.ErrNotFound):
			writeResponse(r.Context(), w, SuccessResponse{Message: "Roamer deregistered successfully"}, http.StatusOK, logger.APILog)
		case errors.Is(err, errRoamerStillRegistered):
			writeError(r.Context(), w, http.StatusConflict, "The roamer is still registered after one minute (the node that holds it may be unreachable); retry later", nil, logger.APILog)
			return
		case err != nil:
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to deregister the roamer", err, logger.APILog)
			return
		default:
			writeResponse(r.Context(), w, SuccessResponse{Message: "Roamer deregistered successfully"}, http.StatusOK, logger.APILog)
		}

		logger.LogAuditEvent(r.Context(), DeregisterRoamerAction, email, getClientIP(r), "Deregistered roamer "+imsi)
	})
}

func deregisterRoamer(ctx context.Context, dbInstance *db.Database, imsi string) error {
	timing := roamerWithdrawalTiming

	if err := dbInstance.WithdrawRoamer(ctx, imsi, time.Now().Add(timing.timeout+timing.margin)); err != nil {
		return err
	}

	defer func() {
		if err := dbInstance.ClearRoamerWithdrawal(context.WithoutCancel(ctx), imsi); err != nil {
			logger.APILog.Warn("failed to clear the withdrawal of a roamer; it expires on its own", logger.SUPIFromIMSI(imsi))
		}
	}()

	deadline := time.Now().Add(timing.timeout)

	for {
		visitor, err := dbInstance.GetSubscriberOrVisitor(ctx, imsi)

		switch {
		case errors.Is(err, db.ErrNotFound) || (err == nil && visitor.RoamingPartnerID == nil):
			return nil
		case err != nil:
			return err
		}

		if time.Now().After(deadline) {
			return errRoamerStillRegistered
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(timing.poll):
		}
	}
}
