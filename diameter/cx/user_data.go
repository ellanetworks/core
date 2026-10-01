// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package cx

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

var ErrInvalidUserData = errors.New("cx: invalid user data")

type IMSSubscription struct {
	PrivateIdentity string
	ServiceProfiles []ServiceProfile
	IMSI            string
}

type ServiceProfile struct {
	PublicIdentities         []ProfileIdentity
	SubscribedMediaProfileID *int32
	InitialFilterCriteria    []InitialFilterCriteria
	SharedIFCSetIDs          []int32
}

type ProfileIdentity struct {
	Identity             string
	Barred               bool
	Type                 IdentityType
	WildcardedPSI        string
	DisplayName          string
	AliasIdentityGroupID string
	WildcardedIMPU       string
}

type InitialFilterCriteria struct {
	Priority          int32
	Trigger           *TriggerPoint
	ApplicationServer ApplicationServer
	ProfilePart       *ProfilePart
}

type TriggerPoint struct {
	ConjunctiveNormalForm bool
	ServicePointTriggers  []ServicePointTrigger
}

type ServicePointTrigger struct {
	Negated            bool
	Groups             []int32
	RequestURI         *string
	Method             *string
	SIPHeader          *HeaderMatch
	SessionCase        *SessionCase
	SessionDescription *SessionDescriptionMatch
	RegistrationTypes  []RegistrationType
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
	DefaultHandling         *DefaultHandling
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
	SubscribedMediaProfileID *int32 `xml:"SubscribedMediaProfileId"`
}

type xmlServiceProfileExtension struct {
	SharedIFCSetID []int32 `xml:"SharedIFCSetID"`
}

type xmlPublicIdentity struct {
	BarringIndication *xmlBool                    `xml:"BarringIndication"`
	Identity          string                      `xml:"Identity"`
	Extension         *xmlPublicIdentityExtension `xml:"Extension"`
}

type xmlPublicIdentityExtension struct {
	IdentityType  *IdentityType                `xml:"IdentityType"`
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
	Priority             *int32               `xml:"Priority"`
	TriggerPoint         *xmlTriggerPoint     `xml:"TriggerPoint"`
	ApplicationServer    xmlApplicationServer `xml:"ApplicationServer"`
	ProfilePartIndicator *ProfilePart         `xml:"ProfilePartIndicator"`
}

type xmlTriggerPoint struct {
	ConditionTypeCNF *xmlBool `xml:"ConditionTypeCNF"`
	SPT              []xmlSPT `xml:"SPT"`
}

type xmlSPT struct {
	ConditionNegated   *xmlBool               `xml:"ConditionNegated"`
	Group              []int32                `xml:"Group"`
	RequestURI         *string                `xml:"RequestURI"`
	Method             *string                `xml:"Method"`
	SIPHeader          *xmlHeader             `xml:"SIPHeader"`
	SessionCase        *SessionCase           `xml:"SessionCase"`
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
	DefaultHandling *DefaultHandling               `xml:"DefaultHandling"`
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

	if err := d.Decode(&w); err != nil {
		return IMSSubscription{}, fmt.Errorf("%w: %w", ErrInvalidUserData, err)
	}

	if err := requireEnd(d); err != nil {
		return IMSSubscription{}, err
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

func requireEnd(d *xml.Decoder) error {
	for {
		tok, err := d.Token()

		switch {
		case errors.Is(err, io.EOF):
			return nil
		case err != nil:
			return fmt.Errorf("%w: %w", ErrInvalidUserData, err)
		}

		switch t := tok.(type) {
		case xml.Comment, xml.ProcInst:
		case xml.CharData:
			if len(bytes.TrimSpace(t)) != 0 {
				return userDataError("content after IMSSubscription")
			}
		default:
			return userDataError("content after IMSSubscription")
		}
	}
}

func validText(name string, values ...string) error {
	for _, v := range values {
		if !utf8.ValidString(v) {
			return userDataError("%s is not UTF-8", name)
		}

		for _, r := range v {
			if r < 0x20 && r != '\t' && r != '\n' && r != '\r' || r == 0xfffe || r == 0xffff {
				return userDataError("%s contains the character %U, which XML does not allow", name, r)
			}
		}
	}

	return nil
}

func validURI(name, v string) error {
	switch {
	case v == "":
		return userDataError("empty %s", name)
	case strings.TrimSpace(v) != v:
		return userDataError("%s %q has surrounding whitespace", name, v)
	}

	return validText(name, v)
}

func optionalURI(name, v string) error {
	if v == "" {
		return nil
	}

	return validURI(name, v)
}

func optionalText(name string, v *string) error {
	if v == nil {
		return nil
	}

	return validText(name, *v)
}

func userDataError(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrInvalidUserData}, args...)...)
}

func (s IMSSubscription) validate() error {
	if err := validURI("PrivateID", s.PrivateIdentity); err != nil {
		return err
	}

	if err := validText("IMSI", s.IMSI); err != nil {
		return err
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
		if err := id.validate(); err != nil {
			return err
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

func (id ProfileIdentity) validate() error {
	if id.Type > IdentityWildcardedIMPU {
		return userDataError("IdentityType %d", id.Type)
	}

	for _, err := range []error{
		validURI("Identity", id.Identity),
		optionalURI("WildcardedPSI", id.WildcardedPSI),
		optionalURI("WildcardedIMPU", id.WildcardedIMPU),
		validText("DisplayName", id.DisplayName),
		validText("AliasIdentityGroupID", id.AliasIdentityGroupID),
	} {
		if err != nil {
			return err
		}
	}

	return nil
}

func (c InitialFilterCriteria) validate() error {
	if err := validURI("ServerName", c.ApplicationServer.ServerName); err != nil {
		return err
	}

	if err := optionalText("ServiceInfo", c.ApplicationServer.ServiceInfo); err != nil {
		return err
	}

	switch {
	case c.Priority < 0:
		return userDataError("iFC Priority %d", c.Priority)
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

	return t.validateText()
}

func (t ServicePointTrigger) validateText() error {
	checks := []error{optionalText("RequestURI", t.RequestURI), optionalText("Method", t.Method)}

	if h := t.SIPHeader; h != nil {
		checks = append(checks, validText("Header", h.Header), optionalText("Content", h.Content))
	}

	if d := t.SessionDescription; d != nil {
		checks = append(checks, validText("Line", d.Line), optionalText("Content", d.Content))
	}

	for _, err := range checks {
		if err != nil {
			return err
		}
	}

	return nil
}

func (s IMSSubscription) wire() xmlSubscription {
	w := xmlSubscription{PrivateID: s.PrivateIdentity}

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

func (id ProfileIdentity) wire() xmlPublicIdentity {
	w := xmlPublicIdentity{Identity: id.Identity}

	if id.Barred {
		barred := xmlBool(true)
		w.BarringIndication = &barred
	}

	var ext2 *xmlPublicIdentityExtension2

	if id.DisplayName != "" || id.AliasIdentityGroupID != "" || id.WildcardedIMPU != "" {
		ext2 = &xmlPublicIdentityExtension2{DisplayName: id.DisplayName, AliasIdentityGroupID: id.AliasIdentityGroupID}

		if id.WildcardedIMPU != "" {
			ext2.Extension = &xmlPublicIdentityExtension3{WildcardedIMPU: id.WildcardedIMPU}
		}
	}

	if id.Type != IdentityDistinctPublicUserIdentity || id.WildcardedPSI != "" || ext2 != nil {
		t := id.Type
		w.Extension = &xmlPublicIdentityExtension{IdentityType: &t, WildcardedPSI: id.WildcardedPSI, Extension: ext2}
	}

	return w
}

func (c InitialFilterCriteria) wire() xmlInitialFilterCriteria {
	w := xmlInitialFilterCriteria{
		Priority: &c.Priority,
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
		cnf := xmlBool(c.Trigger.ConjunctiveNormalForm)
		tp := &xmlTriggerPoint{ConditionTypeCNF: &cnf}

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
	s := IMSSubscription{PrivateIdentity: strings.TrimSpace(w.PrivateID)}

	if w.Extension != nil {
		s.IMSI = w.Extension.IMSI
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

func (w xmlPublicIdentity) model() ProfileIdentity {
	id := ProfileIdentity{Identity: strings.TrimSpace(w.Identity)}

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
	if w.Priority == nil {
		return InitialFilterCriteria{}, userDataError("iFC without a Priority")
	}

	c := InitialFilterCriteria{
		Priority:    *w.Priority,
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
		if w.TriggerPoint.ConditionTypeCNF == nil {
			return InitialFilterCriteria{}, userDataError("TriggerPoint without a ConditionTypeCNF")
		}

		tp := &TriggerPoint{ConjunctiveNormalForm: bool(*w.TriggerPoint.ConditionTypeCNF)}

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

			t.RegistrationTypes = append(t.RegistrationTypes, RegistrationType(r))
		}
	}

	return t, nil
}
