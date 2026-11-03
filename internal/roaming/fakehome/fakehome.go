// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fakehome

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/ellanetworks/core/internal/ausf"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/roaming/wire"
	"github.com/ellanetworks/core/internal/sqn"
	"github.com/ellanetworks/core/internal/udm"
)

type Subscriber struct {
	K      string
	OPc    string
	SQN    string
	AM     wire.AccessAndMobilitySubscriptionData
	SMFSel *wire.SmfSelectionSubscriptionData
	LCS    *wire.LcsPrivacyData
}

type Request struct {
	Method string
	Path   string
	Query  string
	Body   []byte
}

type authContext struct {
	supiOrSuci string
	removed    bool
}

type failure struct {
	method string
	path   string
	pd     wire.ProblemDetails
}

type Home struct {
	PLMN wire.PlmnID

	mu            sync.Mutex
	subscribers   map[string]*Subscriber
	ausf          *ausf.AUSF
	auths         map[string]*authContext
	nextID        int
	registrations map[string]wire.Amf3GppAccessRegistration
	purged        map[string]bool
	subscriptions map[string]wire.SdmSubscription
	sorAcks       map[string]wire.AcknowledgeInfo
	requests      []Request
	failures      []failure
	confirmations []*string
}

func New(plmn wire.PlmnID) *Home {
	h := &Home{
		PLMN:          plmn,
		subscribers:   map[string]*Subscriber{},
		auths:         map[string]*authContext{},
		registrations: map[string]wire.Amf3GppAccessRegistration{},
		purged:        map[string]bool{},
		subscriptions: map[string]wire.SdmSubscription{},
		sorAcks:       map[string]wire.AcknowledgeInfo{},
	}

	h.ausf = ausf.New(credentials{h: h}, func(string, int) (string, error) {
		return "", fmt.Errorf("no home network key")
	})

	return h
}

func (h *Home) Provision(imsi string, s Subscriber) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.subscribers[imsi] = &s
}

func (h *Home) Update(imsi string, f func(*Subscriber)) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if s, ok := h.subscribers[imsi]; ok {
		f(s)
	}
}

func (h *Home) Fail(method, pathContains string, pd wire.ProblemDetails) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.failures = append(h.failures, failure{method: method, path: pathContains, pd: pd})
}

func (h *Home) Heal() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.failures = nil
}

func (h *Home) Requests() []Request {
	h.mu.Lock()
	defer h.mu.Unlock()

	return append([]Request(nil), h.requests...)
}

func (h *Home) Registration(supi string) (wire.Amf3GppAccessRegistration, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	r, ok := h.registrations[supi]

	return r, ok
}

func (h *Home) Purged(supi string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.purged[supi]
}

func (h *Home) Subscriptions() map[string]wire.SdmSubscription {
	h.mu.Lock()
	defer h.mu.Unlock()

	out := make(map[string]wire.SdmSubscription, len(h.subscriptions))
	for k, v := range h.subscriptions {
		out[k] = v
	}

	return out
}

func (h *Home) Confirmations() []*string {
	h.mu.Lock()
	defer h.mu.Unlock()

	return append([]*string(nil), h.confirmations...)
}

func (h *Home) SoRAck(supi string) (wire.AcknowledgeInfo, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	a, ok := h.sorAcks[supi]

	return a, ok
}

func (h *Home) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var body []byte
	if r.Body != nil {
		body, _ = io.ReadAll(r.Body)
	}

	h.mu.Lock()
	h.requests = append(h.requests, Request{Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery, Body: body})

	for _, f := range h.failures {
		if f.method == r.Method && strings.Contains(r.URL.Path, f.path) {
			h.mu.Unlock()
			writeJSON(w, f.pd.Status, wire.ContentTypeProblem, f.pd)

			return
		}
	}
	h.mu.Unlock()

	segments := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(segments) < 2 {
		http.NotFound(w, r)
		return
	}

	switch segments[0] {
	case "nnrf-disc":
		h.discover(w, r)
	case "nausf-auth":
		h.authenticate(w, r, segments[2:], body)
	case "nudm-uecm":
		h.uecm(w, r, segments[2:], body)
	case "nudm-sdm":
		h.sdm(w, r, segments[2:], body)
	default:
		http.NotFound(w, r)
	}
}

func (h *Home) host(nf string) string {
	return "http://" + h.PLMN.NFFQDN(nf)
}

func (h *Home) discover(w http.ResponseWriter, r *http.Request) {
	target := strings.ToLower(r.URL.Query().Get("target-nf-type"))
	services := map[string][]string{"ausf": {"nausf-auth"}, "udm": {"nudm-uecm", "nudm-sdm"}}

	names, ok := services[target]
	if !ok {
		writeJSON(w, http.StatusOK, wire.ContentTypeJSON, map[string]any{"validityPeriod": 60, "nfInstances": []any{}})
		return
	}

	var svcs []map[string]any

	for i, n := range names {
		version := "v1"
		if n == "nudm-sdm" {
			version = "v2"
		}

		svcs = append(svcs, map[string]any{
			"serviceInstanceId": strconv.Itoa(i),
			"serviceName":       n,
			"versions":          []map[string]string{{"apiVersionInUri": version, "apiFullVersion": version[1:] + ".0.0"}},
			"scheme":            "http",
			"nfServiceStatus":   "REGISTERED",
		})
	}

	profile := map[string]any{
		"nfInstanceId": "00000000-0000-0000-0000-00000000000" + strconv.Itoa(len(target)),
		"nfType":       strings.ToUpper(target),
		"nfStatus":     "REGISTERED",
		"plmnList":     []wire.PlmnID{h.PLMN},
		"fqdn":         h.PLMN.NFFQDN(target),
		"nfServices":   svcs,
	}

	writeJSON(w, http.StatusOK, wire.ContentTypeJSON, map[string]any{"validityPeriod": 60, "nfInstances": []any{profile}})
}

func (h *Home) authenticate(w http.ResponseWriter, r *http.Request, rest []string, body []byte) {
	switch {
	case r.Method == http.MethodPost && len(rest) == 1 && rest[0] == "ue-authentications":
		h.startAuthentication(w, r, body)
	case len(rest) == 3 && rest[2] == "5g-aka-confirmation" && r.Method == http.MethodPut:
		h.confirm(w, r, rest[1], body)
	case len(rest) == 3 && rest[2] == "5g-aka-confirmation" && r.Method == http.MethodDelete:
		h.mu.Lock()
		if a, ok := h.auths[rest[1]]; ok {
			a.removed = true
		}
		h.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	default:
		http.NotFound(w, r)
	}
}

func (h *Home) startAuthentication(w http.ResponseWriter, r *http.Request, body []byte) {
	info, err := wire.Decode[wire.AuthenticationInfo](body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, wire.ContentTypeProblem, wire.ProblemFromError(err))
		return
	}

	sn, err := wire.ParseServingNetworkName(info.ServingNetworkName)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, wire.ContentTypeProblem, wire.ProblemFromError(err))
		return
	}

	var resync *ausf.ResyncInfo
	if info.ResynchronizationInfo != nil {
		resync = &ausf.ResyncInfo{Auts: info.ResynchronizationInfo.Auts}
	}

	result, err := h.ausf.Authenticate(r.Context(), info.SupiOrSuci, models.PlmnID{Mcc: sn.Mcc, Mnc: sn.Mnc}, resync)
	if err != nil {
		writeJSON(w, http.StatusNotFound, wire.ContentTypeProblem, wire.NewProblem(http.StatusNotFound, wire.CauseUserNotFound, err.Error()))
		return
	}

	h.mu.Lock()
	h.nextID++
	id := strconv.Itoa(h.nextID)
	h.auths[id] = &authContext{supiOrSuci: info.SupiOrSuci}
	h.mu.Unlock()

	location := h.host("ausf") + wire.UEAuthenticationsPath() + "/" + id
	w.Header().Set("Location", location)

	writeJSON(w, http.StatusCreated, wire.ContentTypeHAL, map[string]any{
		"authType":   wire.AuthType5GAKA,
		"5gAuthData": map[string]string{"rand": result.Rand, "hxresStar": result.HxresStar, "autn": result.Autn},
		"_links":     map[string]any{wire.LinkRel5GAKA: map[string]string{"href": location + "/5g-aka-confirmation"}},
	})
}

func (h *Home) confirm(w http.ResponseWriter, r *http.Request, id string, body []byte) {
	data, err := wire.Decode[wire.ConfirmationData](body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, wire.ContentTypeProblem, wire.ProblemFromError(err))
		return
	}

	h.mu.Lock()
	a, ok := h.auths[id]
	h.confirmations = append(h.confirmations, data.ResStar)
	h.mu.Unlock()

	if !ok {
		writeJSON(w, http.StatusNotFound, wire.ContentTypeProblem, wire.NewProblem(http.StatusNotFound, wire.CauseContextNotFound, ""))
		return
	}

	if data.ResStar == nil {
		writeJSON(w, http.StatusOK, wire.ContentTypeJSON, wire.ConfirmationDataResponse{AuthResult: wire.AuthResultFailure})
		return
	}

	supi, kseaf, err := h.ausf.Confirm(r.Context(), *data.ResStar, a.supiOrSuci)
	if err != nil {
		writeJSON(w, http.StatusOK, wire.ContentTypeJSON, wire.ConfirmationDataResponse{AuthResult: wire.AuthResultFailure})
		return
	}

	writeJSON(w, http.StatusOK, wire.ContentTypeJSON, wire.ConfirmationDataResponse{AuthResult: wire.AuthResultSuccess, Supi: supi.String(), Kseaf: hex.EncodeToString(kseaf)})
}

func (h *Home) uecm(w http.ResponseWriter, r *http.Request, rest []string, body []byte) {
	if len(rest) != 3 || rest[1] != "registrations" || rest[2] != "amf-3gpp-access" {
		http.NotFound(w, r)
		return
	}

	supi := rest[0]

	switch r.Method {
	case http.MethodPut:
		reg, err := wire.Decode[wire.Amf3GppAccessRegistration](body)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, wire.ContentTypeProblem, wire.ProblemFromError(err))
			return
		}

		h.mu.Lock()

		_, known := h.subscribers[strings.TrimPrefix(supi, "imsi-")]
		if known {
			h.registrations[supi] = reg
			delete(h.purged, supi)
		}
		h.mu.Unlock()

		if !known {
			writeJSON(w, http.StatusNotFound, wire.ContentTypeProblem, wire.NewProblem(http.StatusNotFound, wire.CauseUserNotFound, ""))
			return
		}

		writeJSON(w, http.StatusCreated, wire.ContentTypeJSON, reg)
	case http.MethodPatch:
		mod, err := wire.Decode[wire.Amf3GppAccessRegistrationModification](body)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, wire.ContentTypeProblem, wire.ProblemFromError(err))
			return
		}

		h.mu.Lock()
		reg, ok := h.registrations[supi]
		same := ok && reg.Guami != nil && mod.Guami != nil && reg.Guami.Equal(*mod.Guami)

		if same && mod.PurgeFlag != nil && *mod.PurgeFlag {
			h.purged[supi] = true
		}
		h.mu.Unlock()

		if !same {
			writeJSON(w, http.StatusForbidden, wire.ContentTypeProblem, wire.NewProblem(http.StatusForbidden, wire.CauseInvalidGUAMI, ""))
			return
		}

		w.WriteHeader(http.StatusNoContent)
	default:
		http.NotFound(w, r)
	}
}

func (h *Home) sdm(w http.ResponseWriter, r *http.Request, rest []string, body []byte) {
	if len(rest) == 0 {
		http.NotFound(w, r)
		return
	}

	supi := rest[0]

	h.mu.Lock()
	sub, ok := h.subscribers[strings.TrimPrefix(supi, "imsi-")]

	var snapshot Subscriber
	if ok {
		snapshot = *sub
	}
	h.mu.Unlock()

	if !ok {
		writeJSON(w, http.StatusNotFound, wire.ContentTypeProblem, wire.NewProblem(http.StatusNotFound, wire.CauseUserNotFound, ""))
		return
	}

	switch {
	case len(rest) == 1 && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, wire.ContentTypeJSON, map[string]any{"amData": snapshot.AM, "smfSelData": snapshot.SMFSel})
	case len(rest) == 2 && rest[1] == "lcs-privacy-data" && r.Method == http.MethodGet:
		if snapshot.LCS == nil {
			writeJSON(w, http.StatusNotFound, wire.ContentTypeProblem, wire.NewProblem(http.StatusNotFound, wire.CauseDataNotFound, ""))
			return
		}

		writeJSON(w, http.StatusOK, wire.ContentTypeJSON, snapshot.LCS)
	case len(rest) == 2 && rest[1] == "sdm-subscriptions" && r.Method == http.MethodPost:
		s, err := wire.Decode[wire.SdmSubscription](body)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, wire.ContentTypeProblem, wire.ProblemFromError(err))
			return
		}

		h.mu.Lock()
		h.nextID++
		id := strconv.Itoa(h.nextID)
		s.SubscriptionID = id
		h.subscriptions[id] = s
		h.mu.Unlock()

		w.Header().Set("Location", h.host("udm")+wire.SDMSubscriptionPath(supi, id))
		writeJSON(w, http.StatusCreated, wire.ContentTypeJSON, s)
	case len(rest) == 3 && rest[1] == "sdm-subscriptions" && r.Method == http.MethodDelete:
		h.mu.Lock()
		_, ok := h.subscriptions[rest[2]]
		delete(h.subscriptions, rest[2])
		h.mu.Unlock()

		if !ok {
			writeJSON(w, http.StatusNotFound, wire.ContentTypeProblem, wire.NewProblem(http.StatusNotFound, wire.CauseSubscriptionNotFound, ""))
			return
		}

		w.WriteHeader(http.StatusNoContent)
	case len(rest) == 3 && rest[1] == "am-data" && rest[2] == "sor-ack" && r.Method == http.MethodPut:
		ack, err := wire.Decode[wire.AcknowledgeInfo](body)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, wire.ContentTypeProblem, wire.ProblemFromError(err))
			return
		}

		h.mu.Lock()
		h.sorAcks[supi] = ack
		h.mu.Unlock()

		w.WriteHeader(http.StatusNoContent)
	default:
		http.NotFound(w, r)
	}
}

func writeJSON(w http.ResponseWriter, status int, contentType string, v any) {
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

type credentials struct {
	h *Home
}

func (c credentials) AdvanceSequenceNumber(_ context.Context, imsi, resyncAuts, resyncRand string) (*udm.AdvancedCredentials, error) {
	c.h.mu.Lock()
	defer c.h.mu.Unlock()

	sub, ok := c.h.subscribers[imsi]
	if !ok {
		return nil, udm.ErrSubscriberUnknown
	}

	next, err := sqn.Next(sub.SQN, sub.OPc, sub.K, resyncAuts, resyncRand)
	if err != nil {
		return nil, err
	}

	sub.SQN = next

	return &udm.AdvancedCredentials{PermanentKey: sub.K, Opc: sub.OPc, SequenceNumber: next}, nil
}
