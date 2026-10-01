// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package cx

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"strings"
)

var ErrInvalidUserData = errors.New("cx: invalid user data")

const (
	IdentityDistinctPublicUserIdentity uint8 = 0
	IdentityDistinctPSI                uint8 = 1
	IdentityWildcardedPSI              uint8 = 2
	IdentityNonDistinctIMPU            uint8 = 3
	IdentityWildcardedIMPU             uint8 = 4
)

const (
	ProfilePartRegistered   uint8 = 0
	ProfilePartUnregistered uint8 = 1
)

const (
	SessionCaseOriginatingRegistered   uint8 = 0
	SessionCaseTerminatingRegistered   uint8 = 1
	SessionCaseTerminatingUnregistered uint8 = 2
	SessionCaseOriginatingUnregistered uint8 = 3
	SessionCaseOriginatingCDIV         uint8 = 4
)

const (
	RegistrationTypeInitial        uint8 = 0
	RegistrationTypeReRegistration uint8 = 1
	RegistrationTypeDeregistration uint8 = 2
)

const (
	DefaultHandlingSessionContinued  uint8 = 0
	DefaultHandlingSessionTerminated uint8 = 1
)

type IMSSubscription struct {
	PrivateID       string
	ServiceProfiles []ServiceProfile
	IMSI            string
}

type ServiceProfile struct {
	PublicIdentities         []PublicIdentity
	SubscribedMediaProfileID *int
	InitialFilterCriteria    []InitialFilterCriteria
	SharedIFCSetIDs          []int
}

type PublicIdentity struct {
	Identity             string
	Barred               bool
	Type                 uint8
	WildcardedPSI        string
	DisplayName          string
	AliasIdentityGroupID string
	WildcardedIMPU       string
}

type InitialFilterCriteria struct {
	Priority          int
	Trigger           *TriggerPoint
	ApplicationServer ApplicationServer
	ProfilePart       *uint8
}

type TriggerPoint struct {
	ConjunctiveNormalForm bool
	ServicePointTriggers  []ServicePointTrigger
}

type ServicePointTrigger struct {
	Negated            bool
	Groups             []int
	RequestURI         *string
	Method             *string
	SIPHeader          *HeaderMatch
	SessionCase        *uint8
	SessionDescription *SessionDescriptionMatch
	RegistrationTypes  []uint8
}

type HeaderMatch struct {
	Header  string
	Content *string
}

type SessionDescriptionMatch struct {
	Line    string
	Content *string
}

type ApplicationServer struct {
	ServerName              string
	DefaultHandling         *uint8
	ServiceInfo             *string
	IncludeRegisterRequest  bool
	IncludeRegisterResponse bool
}

type xmlBool bool

func (b xmlBool) MarshalText() ([]byte, error) {
	if b {
		return []byte("1"), nil
	}

	return []byte("0"), nil
}

func (b *xmlBool) UnmarshalText(text []byte) error {
	switch strings.TrimSpace(string(text)) {
	case "1", "true":
		*b = true
	case "0", "false":
		*b = false
	default:
		return fmt.Errorf("boolean %q", text)
	}

	return nil
}

type xmlSubscription struct {
	XMLName        xml.Name                  `xml:"IMSSubscription"`
	PrivateID      string                    `xml:"PrivateID"`
	ServiceProfile []xmlServiceProfile       `xml:"ServiceProfile"`
	Extension      *xmlSubscriptionExtension `xml:"Extension"`
}

type xmlSubscriptionExtension struct {
	IMSI string `xml:"IMSI,omitempty"`
}

type xmlServiceProfile struct {
	PublicIdentity                   []xmlPublicIdentity         `xml:"PublicIdentity"`
	CoreNetworkServicesAuthorization *xmlCNServicesAuthorization `xml:"CoreNetworkServicesAuthorization"`
	InitialFilterCriteria            []xmlInitialFilterCriteria  `xml:"InitialFilterCriteria"`
	Extension                        *xmlServiceProfileExtension `xml:"Extension"`
}

type xmlCNServicesAuthorization struct {
	SubscribedMediaProfileID *int `xml:"SubscribedMediaProfileId"`
}

type xmlServiceProfileExtension struct {
	SharedIFCSetID []int `xml:"SharedIFCSetID"`
}

type xmlPublicIdentity struct {
	BarringIndication *xmlBool                    `xml:"BarringIndication"`
	Identity          string                      `xml:"Identity"`
	Extension         *xmlPublicIdentityExtension `xml:"Extension"`
}

type xmlPublicIdentityExtension struct {
	IdentityType  *uint8                       `xml:"IdentityType"`
	WildcardedPSI string                       `xml:"WildcardedPSI,omitempty"`
	Extension     *xmlPublicIdentityExtension2 `xml:"Extension"`
}

type xmlPublicIdentityExtension2 struct {
	DisplayName          string                       `xml:"DisplayName,omitempty"`
	AliasIdentityGroupID string                       `xml:"AliasIdentityGroupID,omitempty"`
	Extension            *xmlPublicIdentityExtension3 `xml:"Extension"`
}

type xmlPublicIdentityExtension3 struct {
	WildcardedIMPU string `xml:"WildcardedIMPU,omitempty"`
}

type xmlInitialFilterCriteria struct {
	Priority             int                  `xml:"Priority"`
	TriggerPoint         *xmlTriggerPoint     `xml:"TriggerPoint"`
	ApplicationServer    xmlApplicationServer `xml:"ApplicationServer"`
	ProfilePartIndicator *uint8               `xml:"ProfilePartIndicator"`
}

type xmlTriggerPoint struct {
	ConditionTypeCNF xmlBool  `xml:"ConditionTypeCNF"`
	SPT              []xmlSPT `xml:"SPT"`
}

type xmlSPT struct {
	ConditionNegated   *xmlBool               `xml:"ConditionNegated"`
	Group              []int                  `xml:"Group"`
	RequestURI         *string                `xml:"RequestURI"`
	Method             *string                `xml:"Method"`
	SIPHeader          *xmlHeader             `xml:"SIPHeader"`
	SessionCase        *uint8                 `xml:"SessionCase"`
	SessionDescription *xmlSessionDescription `xml:"SessionDescription"`
	Extension          *xmlSPTExtension       `xml:"Extension"`
}

type xmlHeader struct {
	Header  string  `xml:"Header"`
	Content *string `xml:"Content"`
}

type xmlSessionDescription struct {
	Line    string  `xml:"Line"`
	Content *string `xml:"Content"`
}

type xmlSPTExtension struct {
	RegistrationType []int `xml:"RegistrationType"`
}

type xmlApplicationServer struct {
	ServerName      string                         `xml:"ServerName"`
	DefaultHandling *uint8                         `xml:"DefaultHandling"`
	ServiceInfo     *string                        `xml:"ServiceInfo"`
	Extension       *xmlApplicationServerExtension `xml:"Extension"`
}

type xmlApplicationServerExtension struct {
	IncludeRegisterRequest  *struct{} `xml:"IncludeRegisterRequest"`
	IncludeRegisterResponse *struct{} `xml:"IncludeRegisterResponse"`
}

func MarshalUserData(s IMSSubscription) ([]byte, error) {
	if err := s.validate(); err != nil {
		return nil, err
	}

	var buf bytes.Buffer

	buf.WriteString(xml.Header)

	if err := xml.NewEncoder(&buf).Encode(s.wire()); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidUserData, err)
	}

	return buf.Bytes(), nil
}

func ParseUserData(b []byte) (IMSSubscription, error) {
	var w xmlSubscription

	d := xml.NewDecoder(bytes.NewReader(b))
	d.Strict = true

	if err := d.Decode(&w); err != nil {
		return IMSSubscription{}, fmt.Errorf("%w: %w", ErrInvalidUserData, err)
	}

	s, err := w.model()
	if err != nil {
		return IMSSubscription{}, err
	}

	if err := s.validate(); err != nil {
		return IMSSubscription{}, err
	}

	return s, nil
}

func userDataError(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrInvalidUserData}, args...)...)
}

func (s IMSSubscription) validate() error {
	if strings.TrimSpace(s.PrivateID) == "" {
		return userDataError("no PrivateID")
	}

	if len(s.ServiceProfiles) == 0 {
		return userDataError("no ServiceProfile")
	}

	for i, p := range s.ServiceProfiles {
		if err := p.validate(); err != nil {
			return fmt.Errorf("%w (service profile %d)", err, i)
		}
	}

	return nil
}

func (p ServiceProfile) validate() error {
	if len(p.PublicIdentities) == 0 {
		return userDataError("no PublicIdentity")
	}

	for _, id := range p.PublicIdentities {
		switch {
		case strings.TrimSpace(id.Identity) == "":
			return userDataError("empty Identity")
		case id.Type > IdentityWildcardedIMPU:
			return userDataError("IdentityType %d", id.Type)
		}
	}

	if p.SubscribedMediaProfileID != nil && *p.SubscribedMediaProfileID < 0 {
		return userDataError("SubscribedMediaProfileId %d", *p.SubscribedMediaProfileID)
	}

	for _, id := range p.SharedIFCSetIDs {
		if id < 0 {
			return userDataError("SharedIFCSetID %d", id)
		}
	}

	for _, ifc := range p.InitialFilterCriteria {
		if err := ifc.validate(); err != nil {
			return err
		}
	}

	return nil
}

func (c InitialFilterCriteria) validate() error {
	switch {
	case c.Priority < 0:
		return userDataError("iFC Priority %d", c.Priority)
	case strings.TrimSpace(c.ApplicationServer.ServerName) == "":
		return userDataError("iFC without an application server")
	case c.ApplicationServer.DefaultHandling != nil && *c.ApplicationServer.DefaultHandling > DefaultHandlingSessionTerminated:
		return userDataError("DefaultHandling %d", *c.ApplicationServer.DefaultHandling)
	case c.ProfilePart != nil && *c.ProfilePart > ProfilePartUnregistered:
		return userDataError("ProfilePartIndicator %d", *c.ProfilePart)
	case c.Trigger != nil && len(c.Trigger.ServicePointTriggers) == 0:
		return userDataError("TriggerPoint without an SPT")
	}

	if c.Trigger == nil {
		return nil
	}

	for _, spt := range c.Trigger.ServicePointTriggers {
		if err := spt.validate(); err != nil {
			return err
		}
	}

	return nil
}

func (t ServicePointTrigger) validate() error {
	conditions := 0

	for _, set := range []bool{t.RequestURI != nil, t.Method != nil, t.SIPHeader != nil, t.SessionCase != nil, t.SessionDescription != nil} {
		if set {
			conditions++
		}
	}

	switch {
	case conditions != 1:
		return userDataError("SPT with %d conditions, want one", conditions)
	case len(t.Groups) == 0:
		return userDataError("SPT without a Group")
	case t.SessionCase != nil && *t.SessionCase > SessionCaseOriginatingCDIV:
		return userDataError("SessionCase %d", *t.SessionCase)
	case len(t.RegistrationTypes) > 2:
		return userDataError("SPT with %d RegistrationType elements", len(t.RegistrationTypes))
	}

	for _, g := range t.Groups {
		if g < 0 {
			return userDataError("SPT Group %d", g)
		}
	}

	for _, r := range t.RegistrationTypes {
		if r > RegistrationTypeDeregistration {
			return userDataError("RegistrationType %d", r)
		}
	}

	return nil
}

func (s IMSSubscription) wire() xmlSubscription {
	w := xmlSubscription{PrivateID: s.PrivateID}

	if s.IMSI != "" {
		w.Extension = &xmlSubscriptionExtension{IMSI: s.IMSI}
	}

	for _, p := range s.ServiceProfiles {
		wp := xmlServiceProfile{}

		for _, id := range p.PublicIdentities {
			wp.PublicIdentity = append(wp.PublicIdentity, id.wire())
		}

		if p.SubscribedMediaProfileID != nil {
			wp.CoreNetworkServicesAuthorization = &xmlCNServicesAuthorization{SubscribedMediaProfileID: p.SubscribedMediaProfileID}
		}

		for _, c := range p.InitialFilterCriteria {
			wp.InitialFilterCriteria = append(wp.InitialFilterCriteria, c.wire())
		}

		if len(p.SharedIFCSetIDs) > 0 {
			wp.Extension = &xmlServiceProfileExtension{SharedIFCSetID: p.SharedIFCSetIDs}
		}

		w.ServiceProfile = append(w.ServiceProfile, wp)
	}

	return w
}

func (id PublicIdentity) wire() xmlPublicIdentity {
	t := id.Type
	w := xmlPublicIdentity{
		Identity:  id.Identity,
		Extension: &xmlPublicIdentityExtension{IdentityType: &t, WildcardedPSI: id.WildcardedPSI},
	}

	if id.Barred {
		barred := xmlBool(true)
		w.BarringIndication = &barred
	}

	if id.DisplayName != "" || id.AliasIdentityGroupID != "" || id.WildcardedIMPU != "" {
		ext2 := &xmlPublicIdentityExtension2{DisplayName: id.DisplayName, AliasIdentityGroupID: id.AliasIdentityGroupID}

		if id.WildcardedIMPU != "" {
			ext2.Extension = &xmlPublicIdentityExtension3{WildcardedIMPU: id.WildcardedIMPU}
		}

		w.Extension.Extension = ext2
	}

	return w
}

func (c InitialFilterCriteria) wire() xmlInitialFilterCriteria {
	w := xmlInitialFilterCriteria{
		Priority: c.Priority,
		ApplicationServer: xmlApplicationServer{
			ServerName:      c.ApplicationServer.ServerName,
			DefaultHandling: c.ApplicationServer.DefaultHandling,
			ServiceInfo:     c.ApplicationServer.ServiceInfo,
		},
		ProfilePartIndicator: c.ProfilePart,
	}

	if as := c.ApplicationServer; as.IncludeRegisterRequest || as.IncludeRegisterResponse {
		ext := &xmlApplicationServerExtension{}

		if as.IncludeRegisterRequest {
			ext.IncludeRegisterRequest = &struct{}{}
		}

		if as.IncludeRegisterResponse {
			ext.IncludeRegisterResponse = &struct{}{}
		}

		w.ApplicationServer.Extension = ext
	}

	if c.Trigger != nil {
		tp := &xmlTriggerPoint{ConditionTypeCNF: xmlBool(c.Trigger.ConjunctiveNormalForm)}

		for _, spt := range c.Trigger.ServicePointTriggers {
			tp.SPT = append(tp.SPT, spt.wire())
		}

		w.TriggerPoint = tp
	}

	return w
}

func (t ServicePointTrigger) wire() xmlSPT {
	negated := xmlBool(t.Negated)

	w := xmlSPT{
		ConditionNegated: &negated,
		Group:            t.Groups,
		RequestURI:       t.RequestURI,
		Method:           t.Method,
		SessionCase:      t.SessionCase,
	}

	if t.SIPHeader != nil {
		w.SIPHeader = &xmlHeader{Header: t.SIPHeader.Header, Content: t.SIPHeader.Content}
	}

	if t.SessionDescription != nil {
		w.SessionDescription = &xmlSessionDescription{Line: t.SessionDescription.Line, Content: t.SessionDescription.Content}
	}

	if len(t.RegistrationTypes) > 0 {
		w.Extension = &xmlSPTExtension{}

		for _, r := range t.RegistrationTypes {
			w.Extension.RegistrationType = append(w.Extension.RegistrationType, int(r))
		}
	}

	return w
}

func (w xmlSubscription) model() (IMSSubscription, error) {
	s := IMSSubscription{PrivateID: strings.TrimSpace(w.PrivateID)}

	if w.Extension != nil {
		s.IMSI = strings.TrimSpace(w.Extension.IMSI)
	}

	for _, wp := range w.ServiceProfile {
		p := ServiceProfile{}

		for _, wid := range wp.PublicIdentity {
			p.PublicIdentities = append(p.PublicIdentities, wid.model())
		}

		if wp.CoreNetworkServicesAuthorization != nil {
			p.SubscribedMediaProfileID = wp.CoreNetworkServicesAuthorization.SubscribedMediaProfileID
		}

		for _, wc := range wp.InitialFilterCriteria {
			c, err := wc.model()
			if err != nil {
				return IMSSubscription{}, err
			}

			p.InitialFilterCriteria = append(p.InitialFilterCriteria, c)
		}

		if wp.Extension != nil {
			p.SharedIFCSetIDs = wp.Extension.SharedIFCSetID
		}

		s.ServiceProfiles = append(s.ServiceProfiles, p)
	}

	return s, nil
}

func (w xmlPublicIdentity) model() PublicIdentity {
	id := PublicIdentity{Identity: strings.TrimSpace(w.Identity)}

	if w.BarringIndication != nil {
		id.Barred = bool(*w.BarringIndication)
	}

	ext := w.Extension
	if ext == nil {
		return id
	}

	if ext.IdentityType != nil {
		id.Type = *ext.IdentityType
	}

	id.WildcardedPSI = strings.TrimSpace(ext.WildcardedPSI)

	if ext2 := ext.Extension; ext2 != nil {
		id.DisplayName = ext2.DisplayName
		id.AliasIdentityGroupID = ext2.AliasIdentityGroupID

		if ext2.Extension != nil {
			id.WildcardedIMPU = strings.TrimSpace(ext2.Extension.WildcardedIMPU)
		}
	}

	return id
}

func (w xmlInitialFilterCriteria) model() (InitialFilterCriteria, error) {
	c := InitialFilterCriteria{
		Priority:    w.Priority,
		ProfilePart: w.ProfilePartIndicator,
		ApplicationServer: ApplicationServer{
			ServerName:      strings.TrimSpace(w.ApplicationServer.ServerName),
			DefaultHandling: w.ApplicationServer.DefaultHandling,
			ServiceInfo:     w.ApplicationServer.ServiceInfo,
		},
	}

	if ext := w.ApplicationServer.Extension; ext != nil {
		c.ApplicationServer.IncludeRegisterRequest = ext.IncludeRegisterRequest != nil
		c.ApplicationServer.IncludeRegisterResponse = ext.IncludeRegisterResponse != nil
	}

	if w.TriggerPoint != nil {
		tp := &TriggerPoint{ConjunctiveNormalForm: bool(w.TriggerPoint.ConditionTypeCNF)}

		for _, ws := range w.TriggerPoint.SPT {
			spt, err := ws.model()
			if err != nil {
				return InitialFilterCriteria{}, err
			}

			tp.ServicePointTriggers = append(tp.ServicePointTriggers, spt)
		}

		c.Trigger = tp
	}

	return c, nil
}

func (w xmlSPT) model() (ServicePointTrigger, error) {
	t := ServicePointTrigger{
		Groups:      w.Group,
		RequestURI:  w.RequestURI,
		Method:      w.Method,
		SessionCase: w.SessionCase,
	}

	if w.ConditionNegated != nil {
		t.Negated = bool(*w.ConditionNegated)
	}

	if w.SIPHeader != nil {
		t.SIPHeader = &HeaderMatch{Header: w.SIPHeader.Header, Content: w.SIPHeader.Content}
	}

	if w.SessionDescription != nil {
		t.SessionDescription = &SessionDescriptionMatch{Line: w.SessionDescription.Line, Content: w.SessionDescription.Content}
	}

	if w.Extension != nil {
		for _, r := range w.Extension.RegistrationType {
			if r < 0 || r > int(RegistrationTypeDeregistration) {
				return ServicePointTrigger{}, userDataError("RegistrationType %d", r)
			}

			t.RegistrationTypes = append(t.RegistrationTypes, uint8(r))
		}
	}

	return t, nil
}
