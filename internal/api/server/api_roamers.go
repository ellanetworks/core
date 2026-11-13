// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/smf"
)

const DeregisterRoamerAction = "deregister_roamer"

type Roamer struct {
	Imsi     string           `json:"imsi"`
	Partner  string           `json:"partner"`
	HomePLMN *RoamingPLMN     `json:"home_plmn,omitempty"`
	Msisdn   string           `json:"msisdn,omitempty"`
	Since    string           `json:"since"`
	Status   SubscriberStatus `json:"status"`
}

type ListRoamersResponse struct {
	Items      []Roamer `json:"items"`
	Page       int      `json:"page"`
	PerPage    int      `json:"per_page"`
	TotalCount int      `json:"total_count"`
}

type RoamerDetail struct {
	Imsi          string         `json:"imsi"`
	Partner       string         `json:"partner"`
	HomePLMN      *RoamingPLMN   `json:"home_plmn,omitempty"`
	Msisdn        string         `json:"msisdn,omitempty"`
	Since         string         `json:"since"`
	Registrations []Registration `json:"registrations"`
	Sessions      []Session      `json:"sessions"`
}

type roamerPartners map[string]db.RoamingPartner

func listRoamerPartners(ctx context.Context, dbInstance *db.Database) (roamerPartners, error) {
	partners, err := dbInstance.ListRoamingPartners(ctx)
	if err != nil {
		return nil, err
	}

	byID := make(roamerPartners, len(partners))
	for _, p := range partners {
		byID[p.ID] = p
	}

	return byID, nil
}

func (p roamerPartners) byName(name string) (db.RoamingPartner, bool) {
	for _, partner := range p {
		if partner.Name == name {
			return partner, true
		}
	}

	return db.RoamingPartner{}, false
}

func (p roamerPartners) describe(v db.Visitor) (string, *RoamingPLMN) {
	partner, ok := p[v.PartnerID]
	if !ok {
		return "", nil
	}

	plmn, ok := partner.PLMNOfIMSI(v.Imsi)
	if !ok {
		return partner.Name, nil
	}

	return partner.Name, &RoamingPLMN{Mcc: plmn.Mcc, Mnc: plmn.Mnc}
}

func formatSince(since int64) string {
	return time.Unix(since, 0).UTC().Format(time.RFC3339)
}

type liveStatus struct {
	amf5G         map[string]amf.ConnectedSubscriber
	amf5GLastSeen map[string]amf.LastSeen
	mme4G         map[string]mme.ConnectedSubscriber
	mmeLastSeen   map[string]mme.LastSeen
}

func snapshotLiveStatus(amfInstance *amf.AMF, mmeInstance *mme.MME) liveStatus {
	s := liveStatus{
		amf5G:         amfInstance.ConnectedSubscribers(),
		amf5GLastSeen: amfInstance.LastSeenAll(),
	}

	if mmeInstance != nil {
		s.mme4G = mmeInstance.ConnectedSubscribers()
		s.mmeLastSeen = mmeInstance.LastSeenAll()
	}

	return s
}

func (s liveStatus) of(imsi string) SubscriberStatus {
	amf5G, on5G := s.amf5G[imsi]
	mme4G, on4G := s.mme4G[imsi]

	merged := mergeSystems(
		systemView{
			system: System5G, present: on5G, registered: amf5G.Registered, connected: amf5G.Connected,
			lastSeenAt:    lastSeenAt(on5G, amf5G.LastSeenAt, s.amf5GLastSeen[imsi].At),
			lastSeenRadio: s.amf5GLastSeen[imsi].RadioName,
		},
		systemView{
			system: System4G, present: on4G, registered: mme4G.Registered, connected: mme4G.Connected,
			lastSeenAt:    lastSeenAt(on4G, mme4G.LastSeenAt, s.mmeLastSeen[imsi].At),
			lastSeenRadio: s.mmeLastSeen[imsi].RadioName,
		},
	)

	status := SubscriberStatus{
		Registered:      merged.Registered,
		ConnectionState: connectionState(on5G || on4G, merged.Connected),
		Systems:         merged.Systems,
		NumSessions:     mme4G.NumSessions + amf5G.NumSessions,
		LastSeenRadio:   merged.LastSeenRadio,
	}

	if !merged.LastSeenAt.IsZero() {
		status.LastSeenAt = merged.LastSeenAt.UTC().Format(time.RFC3339)
	}

	return status
}

func (s liveStatus) onRadio(radio string) map[string]struct{} {
	imsis := map[string]struct{}{}

	for imsi := range s.amf5G {
		if s.amf5GLastSeen[imsi].RadioName == radio {
			imsis[imsi] = struct{}{}
		}
	}

	for imsi := range s.mme4G {
		if s.mmeLastSeen[imsi].RadioName == radio {
			imsis[imsi] = struct{}{}
		}
	}

	return imsis
}

func ListRoamers(dbInstance *db.Database, amfInstance *amf.AMF, mmeInstance *mme.MME) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		q := r.URL.Query()
		page := atoiDefault(q.Get("page"), 1)
		perPage := atoiDefault(q.Get("per_page"), 25)
		partnerFilter := q.Get("partner")
		searchFilter := strings.TrimSpace(q.Get("search"))
		radioFilter := q.Get("radio")
		dataNetworkFilter := q.Get("data_network")

		if page < 1 {
			writeError(ctx, w, http.StatusBadRequest, "page must be >= 1", nil, logger.APILog)
			return
		}

		if perPage < 1 || perPage > 100 {
			writeError(ctx, w, http.StatusBadRequest, "per_page must be between 1 and 100", nil, logger.APILog)
			return
		}

		if utf8.RuneCountInString(searchFilter) > MaxSearchLength {
			writeError(ctx, w, http.StatusBadRequest, fmt.Sprintf("search must be at most %d characters", MaxSearchLength), nil, logger.APILog)
			return
		}

		partners, err := listRoamerPartners(ctx, dbInstance)
		if err != nil {
			writeError(ctx, w, http.StatusInternalServerError, "Failed to list roaming partners", err, logger.APILog)
			return
		}

		filters := &db.VisitorFilters{}

		if partnerFilter != "" {
			partner, ok := partners.byName(partnerFilter)
			if !ok {
				writeError(ctx, w, http.StatusNotFound, "Roaming partner not found", nil, logger.APILog)
				return
			}

			filters.PartnerID = &partner.ID
		}

		if searchFilter != "" {
			filters.Search = &searchFilter
		}

		if dataNetworkFilter != "" {
			dn, err := dbInstance.GetDataNetwork(ctx, dataNetworkFilter)
			if err != nil {
				writeError(ctx, w, http.StatusNotFound, "Data Network not found", nil, logger.APILog)
				return
			}

			filters.DataNetworkID = &dn.ID
		}

		live := snapshotLiveStatus(amfInstance, mmeInstance)

		var onRadio map[string]struct{}

		dbPage, dbPerPage := page, perPage

		if radioFilter != "" {
			if !radioIsKnown(amfInstance, mmeInstance, radioFilter) {
				writeError(ctx, w, http.StatusNotFound, "Radio not found", fmt.Errorf("radio %q not found", radioFilter), logger.APILog)
				return
			}

			onRadio = live.onRadio(radioFilter)
			dbPage, dbPerPage = 1, MaxNumSubscribers
		}

		visitors, total, err := dbInstance.ListVisitorsPage(ctx, filters, dbPage, dbPerPage)
		if err != nil {
			writeError(ctx, w, http.StatusInternalServerError, "Failed to list roamers", err, logger.APILog)
			return
		}

		if onRadio != nil {
			visitors = slices.DeleteFunc(visitors, func(v db.Visitor) bool {
				_, ok := onRadio[v.Imsi]
				return !ok
			})
			total = len(visitors)
			visitors = visitors[min((page-1)*perPage, total):min(page*perPage, total)]
		}

		items := make([]Roamer, 0, len(visitors))

		for _, v := range visitors {
			name, homePLMN := partners.describe(v)

			items = append(items, Roamer{
				Imsi:     v.Imsi,
				Partner:  name,
				HomePLMN: homePLMN,
				Msisdn:   formatE164(v.Msisdn),
				Since:    formatSince(v.Since),
				Status:   live.of(v.Imsi),
			})
		}

		writeResponse(ctx, w, ListRoamersResponse{Items: items, Page: page, PerPage: perPage, TotalCount: total}, http.StatusOK, logger.APILog)
	})
}

func GetRoamer(dbInstance *db.Database, amfInstance *amf.AMF, mmeInstance *mme.MME, sessionQuerier smf.SessionQuerier) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		imsi := r.PathValue("imsi")

		supi, err := etsi.NewSUPIFromIMSI(imsi)
		if err != nil {
			writeError(ctx, w, http.StatusBadRequest, "Invalid IMSI format", err, logger.APILog)
			return
		}

		visitor, err := dbInstance.GetVisitor(ctx, imsi)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(ctx, w, http.StatusNotFound, "Roamer not found", nil, logger.APILog)
				return
			}

			writeError(ctx, w, http.StatusInternalServerError, "Failed to retrieve roamer", err, logger.APILog)

			return
		}

		partners, err := listRoamerPartners(ctx, dbInstance)
		if err != nil {
			writeError(ctx, w, http.StatusInternalServerError, "Failed to list roaming partners", err, logger.APILog)
			return
		}

		name, homePLMN := partners.describe(*visitor)
		registrations, sessions := liveRegistrations(r.Context(), supi, imsi, amfInstance, mmeInstance, sessionQuerier)

		writeResponse(ctx, w, RoamerDetail{
			Imsi:          visitor.Imsi,
			Partner:       name,
			HomePLMN:      homePLMN,
			Msisdn:        formatE164(visitor.Msisdn),
			Since:         formatSince(visitor.Since),
			Registrations: registrations,
			Sessions:      sessions,
		}, http.StatusOK, logger.APILog)
	})
}

func liveRegistrations(ctx context.Context, supi etsi.SUPI, imsi string, amfInstance *amf.AMF, mmeInstance *mme.MME, sessionQuerier smf.SessionQuerier) ([]Registration, []Session) {
	snap, pduSessions, found := amfInstance.LookupSubscriber(supi)

	var (
		cs               mme.ConnectedSubscriber
		on4G             bool
		mmeLastSeen      mme.LastSeen
		mmeLastSeenKnown bool
	)

	if mmeInstance != nil {
		cs, on4G = mmeInstance.LookupSubscriber(imsi)
		mmeLastSeen, mmeLastSeenKnown = mmeInstance.LastSeen(imsi)
	}

	amfLastSeen, amfLastSeenKnown := amfInstance.LastSeen(imsi)

	registrations := make([]Registration, 0, 2)

	if reg, ok := registrationFrom5G(snap, found, amfLastSeen, amfLastSeenKnown); ok {
		registrations = append(registrations, reg)
	}

	if reg, ok := registrationFrom4G(cs, on4G, mmeLastSeen, mmeLastSeenKnown); ok {
		registrations = append(registrations, reg)
	}

	sessions := make([]Session, 0, len(pduSessions)+len(cs.Sessions))

	if found {
		for _, pdu := range pduSessions {
			if len(sessions) >= MaxSessions {
				break
			}

			sessions = append(sessions, withSessionState(ctx, sessionFrom5G(pdu), sessionQuerier, pdu.Ref))
		}
	}

	for i := range cs.Sessions {
		if len(sessions) >= MaxSessions {
			break
		}

		sessions = append(sessions, withSessionState(ctx, sessionFrom4G(&cs.Sessions[i]), sessionQuerier, cs.Sessions[i].SessionRef))
	}

	return registrations, sessions
}

func DeregisterRoamer(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		email := getEmailFromContext(r)

		imsi := r.PathValue("imsi")
		if _, err := etsi.NewSUPIFromIMSI(imsi); err != nil {
			writeError(ctx, w, http.StatusBadRequest, "Invalid IMSI format", err, logger.APILog)
			return
		}

		if _, err := dbInstance.GetVisitor(ctx, imsi); err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(ctx, w, http.StatusNotFound, "Roamer not found", nil, logger.APILog)
				return
			}

			writeError(ctx, w, http.StatusInternalServerError, "Failed to retrieve roamer", err, logger.APILog)

			return
		}

		err := deregisterRoamer(ctx, dbInstance, imsi)

		switch {
		case err == nil || errors.Is(err, db.ErrNotFound):
		case errors.Is(err, errRoamersStillRegistered):
			writeError(ctx, w, http.StatusConflict, "The roamer is still registered after one minute (the node that holds it may be unreachable); retry later", nil, logger.APILog)
			return
		default:
			writeError(ctx, w, http.StatusInternalServerError, "Failed to deregister the roamer", err, logger.APILog)
			return
		}

		writeResponse(ctx, w, SuccessResponse{Message: "Roamer deregistered successfully"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(ctx, DeregisterRoamerAction, email, getClientIP(r), "Deregistered roamer "+imsi)
	})
}

func deregisterRoamer(ctx context.Context, dbInstance *db.Database, imsi string) error {
	ctx = context.WithoutCancel(ctx)

	mark := func(ctx context.Context, until time.Time) error {
		return dbInstance.SetVisitorDeregistering(ctx, imsi, until)
	}

	err := withdrawRoamers(ctx, mark, func(ctx context.Context) (int, error) {
		_, err := dbInstance.GetVisitor(ctx, imsi)
		if errors.Is(err, db.ErrNotFound) {
			return 0, nil
		}

		return 1, err
	})
	if err != nil {
		clearWithdrawal(ctx, mark, imsi)
	}

	return err
}
