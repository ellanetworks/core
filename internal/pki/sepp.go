// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package pki

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"slices"
	"strings"
	"time"
)

const (
	SEPPCertValidityYears = 3
	DefaultSEPPID         = "1"
	wellKnownSEPPHost     = "sepp"
	minRSAExponent        = 65537
)

const (
	SEPPWarningLongValidity     = "The certificate is valid for more than 3 years, the maximum of the TS 33.310 clause 6.1.3c.3 profile"
	SEPPWarningNoEKU            = "The certificate has no extendedKeyUsage; TS 33.310 clause 6.1.3c.3 requires serverAuth and clientAuth"
	SEPPWarningNoNFTypes        = "The certificate has no nfTypes extension naming SEPP (RFC 9310), which TS 33.310 clause 6.1.3c.3 requires"
	SEPPWarningCNNotInSAN       = "The certificate's subject CN is not one of its subjectAltName DNS names (GSMA FS.34 clause 5)"
	SEPPWarningHostNotSEPPID    = "The SEPP FQDN does not start with sepp<SEPPID>, the GSMA FS.34 clause 5.1.1 naming"
	SEPPWarningNoCRLDistributor = "The certificate has no cRLDistributionPoints, which TS 33.310 clause 6.1.3c.3 requires: partners can only pin it"
)

var (
	OIDExtensionSubjectAltName = asn1.ObjectIdentifier{2, 5, 29, 17}
	OIDExtensionNFTypes        = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 1, 34}
	oidOrganization            = asn1.ObjectIdentifier{2, 5, 4, 10}
	oidCommonName              = asn1.ObjectIdentifier{2, 5, 4, 3}
)

type PLMN struct {
	MCC string
	MNC string
}

func (p PLMN) PaddedMNC() string {
	if len(p.MNC) == 2 {
		return "0" + p.MNC
	}

	return p.MNC
}

func (p PLMN) Equivalent(o PLMN) bool {
	return p.MCC == o.MCC && p.PaddedMNC() == o.PaddedMNC()
}

func (p PLMN) Domain() string {
	return "5gc.mnc" + p.PaddedMNC() + ".mcc" + p.MCC + ".3gppnetwork.org"
}

func (p PLMN) DefaultSEPPFQDN() string {
	return wellKnownSEPPHost + DefaultSEPPID + "." + p.Domain()
}

func (p PLMN) Validate() error {
	if !allDigits(p.MCC) || len(p.MCC) != 3 {
		return fmt.Errorf("MCC must be 3 digits, got %q", p.MCC)
	}

	if !allDigits(p.MNC) || (len(p.MNC) != 2 && len(p.MNC) != 3) {
		return fmt.Errorf("MNC must be 2 or 3 digits, got %q", p.MNC)
	}

	return nil
}

func allDigits(s string) bool {
	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}

	return s != ""
}

func fqdnLabels(name string) []string {
	return strings.Split(strings.ToLower(strings.TrimSuffix(name, ".")), ".")
}

func plmnLabels(mncLabel, mccLabel string) (PLMN, bool) {
	mnc, okMNC := strings.CutPrefix(mncLabel, "mnc")
	mcc, okMCC := strings.CutPrefix(mccLabel, "mcc")

	if !okMCC || !okMNC || len(mcc) != 3 || len(mnc) != 3 || !allDigits(mcc) || !allDigits(mnc) {
		return PLMN{}, false
	}

	return PLMN{MCC: mcc, MNC: mnc}, true
}

func PLMNOfFQDN(name string) (PLMN, bool) {
	labels := fqdnLabels(name)
	n := len(labels)

	switch {
	case n >= 4 && labels[n-2] == "3gppnetwork" && labels[n-1] == "org":
		return plmnLabels(labels[n-4], labels[n-3])
	case n >= 5 && labels[n-2] == "ipxnetwork" && labels[n-1] == "org":
		return plmnLabels(labels[n-5], labels[n-4])
	}

	return PLMN{}, false
}

type SEPPName struct {
	Host string
	PLMN PLMN
}

func (n SEPPName) WellKnown() bool {
	return n.Host == wellKnownSEPPHost
}

func (n SEPPName) HasSEPPID() bool {
	id, ok := strings.CutPrefix(n.Host, wellKnownSEPPHost)
	return ok && id != "" && !strings.Contains(id, ".")
}

func ParseSEPPName(fqdn string) (SEPPName, error) {
	if fqdn != strings.ToLower(fqdn) {
		return SEPPName{}, fmt.Errorf("FQDN %q must be lowercase", fqdn)
	}

	if len(fqdn) > 253 {
		return SEPPName{}, fmt.Errorf("FQDN %q is longer than 253 characters", fqdn)
	}

	labels := strings.Split(fqdn, ".")
	for _, label := range labels {
		if !validDNSLabel(label) {
			return SEPPName{}, fmt.Errorf("FQDN %q has an invalid label %q", fqdn, label)
		}
	}

	n := len(labels)

	var suffix int

	switch {
	case n >= 6 && labels[n-2] == "3gppnetwork" && labels[n-1] == "org":
		suffix = 5
	case n >= 7 && labels[n-2] == "ipxnetwork" && labels[n-1] == "org":
		suffix = 6
	default:
		return SEPPName{}, fmt.Errorf("FQDN %q must be <host>.5gc.mnc<MNC>.mcc<MCC>.3gppnetwork.org or <host>.5gc.mnc<MNC>.mcc<MCC>.<IPX ID>.ipxnetwork.org", fqdn)
	}

	if labels[n-suffix] != "5gc" {
		return SEPPName{}, fmt.Errorf("FQDN %q must be under the 5GC domain 5gc.mnc<MNC>.mcc<MCC>", fqdn)
	}

	plmn, ok := plmnLabels(labels[n-suffix+1], labels[n-suffix+2])
	if !ok {
		return SEPPName{}, fmt.Errorf("FQDN %q must carry a 3-digit MNC and MCC, mnc<MNC>.mcc<MCC>", fqdn)
	}

	return SEPPName{Host: strings.Join(labels[:n-suffix], "."), PLMN: plmn}, nil
}

func ValidatePeerSEPPFQDN(fqdn string, plmns []PLMN) error {
	name, err := ParseSEPPName(fqdn)
	if err != nil {
		return err
	}

	if name.WellKnown() {
		return fmt.Errorf("FQDN %q is the well-known SEPP FQDN, which must not name a SEPP (GSMA FS.34 clause 5.1.1)", fqdn)
	}

	if !slices.ContainsFunc(plmns, name.PLMN.Equivalent) {
		return fmt.Errorf("FQDN %q names PLMN %s-%s, which is not one of the partner's", fqdn, name.PLMN.MCC, name.PLMN.MNC)
	}

	return nil
}

func ValidateSEPPFQDN(fqdn string, plmn PLMN) error {
	if err := plmn.Validate(); err != nil {
		return err
	}

	domain := plmn.Domain()

	name, err := ParseSEPPName(fqdn)
	if err != nil {
		return err
	}

	if !strings.HasSuffix(fqdn, "."+domain) {
		return fmt.Errorf("FQDN %q must be sepp<SEPPID>.%s", fqdn, domain)
	}

	if !name.HasSEPPID() {
		return fmt.Errorf("FQDN %q must be sepp<SEPPID>.%s, with a SEPP ID (GSMA FS.34 clause 5.1.1)", fqdn, domain)
	}

	return nil
}

func validDNSLabel(label string) bool {
	if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
		return false
	}

	for i := range len(label) {
		c := label[i]
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
			return false
		}
	}

	return true
}

func utf8Subject(organization, commonName string) pkix.Name {
	return pkix.Name{ExtraNames: []pkix.AttributeTypeAndValue{
		{Type: oidOrganization, Value: asn1.RawValue{Tag: asn1.TagUTF8String, Bytes: []byte(organization)}},
		{Type: oidCommonName, Value: asn1.RawValue{Tag: asn1.TagUTF8String, Bytes: []byte(commonName)}},
	}}
}

func GenerateSEPPCert(fqdn string, plmn PLMN) (*x509.Certificate, crypto.Signer, *x509.Certificate, error) {
	if err := ValidateSEPPFQDN(fqdn, plmn); err != nil {
		return nil, nil, nil, err
	}

	domain := plmn.Domain()
	notBefore := time.Now().Add(-time.Minute).UTC().Truncate(time.Second)
	notAfter := notBefore.AddDate(SEPPCertValidityYears, 0, 0)

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("generate SEPP CA key: %w", err)
	}

	caSerial, err := seppSerial()
	if err != nil {
		return nil, nil, nil, err
	}

	caSKID, err := rfc5280KeyID(&caKey.PublicKey)
	if err != nil {
		return nil, nil, nil, err
	}

	caTmpl := &x509.Certificate{
		SerialNumber:          caSerial,
		Subject:               utf8Subject(domain, "SEPP CA"),
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            0,
		MaxPathLenZero:        true,
		SubjectKeyId:          caSKID,
	}

	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create SEPP CA certificate: %w", err)
	}

	ca, err := x509.ParseCertificate(caDER)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("parse SEPP CA certificate: %w", err)
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("generate SEPP key: %w", err)
	}

	serial, err := seppSerial()
	if err != nil {
		return nil, nil, nil, err
	}

	skid, err := rfc5280KeyID(&key.PublicKey)
	if err != nil {
		return nil, nil, nil, err
	}

	san, err := asn1.Marshal([]asn1.RawValue{{Class: asn1.ClassContextSpecific, Tag: 2, Bytes: []byte(fqdn)}})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("encode subjectAltName: %w", err)
	}

	nfTypes, err := asn1.Marshal([]asn1.RawValue{{Class: asn1.ClassUniversal, Tag: asn1.TagIA5String, Bytes: []byte("SEPP")}})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("encode nfTypes: %w", err)
	}

	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      utf8Subject(domain, fqdn),
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		SubjectKeyId: skid,
		ExtraExtensions: []pkix.Extension{
			{Id: OIDExtensionSubjectAltName, Critical: true, Value: san},
			{Id: OIDExtensionNFTypes, Value: nfTypes},
		},
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca, &key.PublicKey, caKey)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create SEPP certificate: %w", err)
	}

	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("parse SEPP certificate: %w", err)
	}

	return leaf, key, ca, nil
}

func seppSerial() (*big.Int, error) {
	limit := new(big.Int).Lsh(big.NewInt(1), 159)
	limit.Sub(limit, big.NewInt(1))

	n, err := rand.Int(rand.Reader, limit)
	if err != nil {
		return nil, fmt.Errorf("random serial: %w", err)
	}

	return n.Add(n, big.NewInt(1)), nil
}

func rfc5280KeyID(pub crypto.PublicKey) ([]byte, error) {
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return nil, fmt.Errorf("marshal public key: %w", err)
	}

	var spki struct {
		Algorithm pkix.AlgorithmIdentifier
		PublicKey asn1.BitString
	}

	if _, err := asn1.Unmarshal(der, &spki); err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}

	sum := sha1.Sum(spki.PublicKey.Bytes)

	return sum[:], nil
}

type SEPPCertificate struct {
	Chain []*x509.Certificate
	FQDN  string
}

func (c SEPPCertificate) Leaf() *x509.Certificate {
	return c.Chain[0]
}

func (c SEPPCertificate) TrustAnchor() *x509.Certificate {
	return c.Chain[len(c.Chain)-1]
}

func (c SEPPCertificate) ChainPEM() string {
	var buf bytes.Buffer

	for _, cert := range c.Chain {
		buf.Write(EncodeCertPEM(cert))
	}

	return buf.String()
}

func (c SEPPCertificate) ProfileWarnings() []string {
	leaf := c.Leaf()

	var warnings []string

	if leaf.NotAfter.After(leaf.NotBefore.AddDate(SEPPCertValidityYears, 0, 0)) {
		warnings = append(warnings, SEPPWarningLongValidity)
	}

	if len(leaf.ExtKeyUsage) == 0 && len(leaf.UnknownExtKeyUsage) == 0 {
		warnings = append(warnings, SEPPWarningNoEKU)
	}

	if !slices.Contains(nfTypesOf(leaf), "SEPP") {
		warnings = append(warnings, SEPPWarningNoNFTypes)
	}

	if cn := leaf.Subject.CommonName; cn != "" && !slices.ContainsFunc(leaf.DNSNames, func(n string) bool { return strings.EqualFold(n, cn) }) {
		warnings = append(warnings, SEPPWarningCNNotInSAN)
	}

	if name, err := ParseSEPPName(c.FQDN); err == nil && !name.HasSEPPID() {
		warnings = append(warnings, SEPPWarningHostNotSEPPID)
	}

	if len(leaf.CRLDistributionPoints) == 0 {
		warnings = append(warnings, SEPPWarningNoCRLDistributor)
	}

	return warnings
}

func nfTypesOf(cert *x509.Certificate) []string {
	for _, ext := range cert.Extensions {
		if !ext.Id.Equal(OIDExtensionNFTypes) {
			continue
		}

		var raw []asn1.RawValue
		if rest, err := asn1.Unmarshal(ext.Value, &raw); err != nil || len(rest) != 0 {
			return nil
		}

		types := make([]string, 0, len(raw))
		for _, v := range raw {
			if v.Class == asn1.ClassUniversal && v.Tag == asn1.TagIA5String {
				types = append(types, string(v.Bytes))
			}
		}

		return types
	}

	return nil
}

func ParseCertChainPEM(data []byte) ([]*x509.Certificate, error) {
	var chain []*x509.Certificate

	rest := data

	for {
		block, remaining := pem.Decode(rest)
		if block == nil {
			break
		}

		rest = remaining

		if block.Type != "CERTIFICATE" {
			return nil, fmt.Errorf("unexpected PEM block %q", block.Type)
		}

		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse certificate %d: %w", len(chain)+1, err)
		}

		chain = append(chain, cert)
	}

	if len(chain) == 0 {
		return nil, errors.New("no PEM certificate found")
	}

	if len(bytes.TrimSpace(rest)) != 0 {
		return nil, errors.New("unexpected data after the last PEM certificate")
	}

	return chain, nil
}

func ParseSEPPCertificate(chainPEM []byte, fqdn string) (SEPPCertificate, error) {
	chain, err := ParseCertChainPEM(chainPEM)
	if err != nil {
		return SEPPCertificate{}, err
	}

	if !slices.ContainsFunc(chain[0].DNSNames, func(n string) bool { return strings.EqualFold(n, fqdn) }) {
		return SEPPCertificate{}, fmt.Errorf("SEPP FQDN %q is not a subjectAltName DNS name of the certificate", fqdn)
	}

	return SEPPCertificate{Chain: chain, FQDN: fqdn}, nil
}

func ValidateSEPPIdentity(chainPEM, keyPEM []byte, fqdn string) (SEPPCertificate, error) {
	if _, err := tls.X509KeyPair(chainPEM, keyPEM); err != nil {
		return SEPPCertificate{}, fmt.Errorf("certificate and private key do not form a key pair: %w", err)
	}

	return ParseSEPPCertificate(chainPEM, fqdn)
}

func ValidateSEPPCertificate(chainPEM, keyPEM []byte, fqdn string, plmn PLMN, now time.Time) (SEPPCertificate, error) {
	if err := plmn.Validate(); err != nil {
		return SEPPCertificate{}, err
	}

	if _, err := tls.X509KeyPair(chainPEM, keyPEM); err != nil {
		return SEPPCertificate{}, fmt.Errorf("certificate and private key do not form a key pair: %w", err)
	}

	chain, err := ParseCertChainPEM(chainPEM)
	if err != nil {
		return SEPPCertificate{}, err
	}

	leaf := chain[0]

	if err := checkSEPPKey(leaf.PublicKey); err != nil {
		return SEPPCertificate{}, err
	}

	if leaf.IsCA {
		return SEPPCertificate{}, errors.New("certificate must be an end-entity certificate, not a CA")
	}

	if leaf.KeyUsage&x509.KeyUsageDigitalSignature == 0 {
		return SEPPCertificate{}, errors.New("certificate keyUsage must include digitalSignature")
	}

	if !hasServerAndClientAuth(leaf) {
		return SEPPCertificate{}, errors.New("certificate extendedKeyUsage must include both serverAuth and clientAuth, or be absent")
	}

	if now.After(leaf.NotAfter) {
		return SEPPCertificate{}, fmt.Errorf("certificate expired on %s", leaf.NotAfter.UTC().Format(time.RFC3339))
	}

	selected, err := seppFQDNOfLeaf(leaf, fqdn, plmn)
	if err != nil {
		return SEPPCertificate{}, err
	}

	if err := verifySEPPChain(chain, now); err != nil {
		return SEPPCertificate{}, err
	}

	return SEPPCertificate{Chain: chain, FQDN: selected}, nil
}

func verifySEPPChain(chain []*x509.Certificate, now time.Time) error {
	if len(chain) < 2 {
		return errors.New("the chain must end with the CA certificate (root or sub-CA) that partners use as Ella Core's trust anchor")
	}

	anchor := chain[len(chain)-1]
	if !isCACert(anchor) {
		return errors.New("the last certificate of the chain must be a CA certificate (root or sub-CA): partners use it as Ella Core's trust anchor")
	}

	roots := x509.NewCertPool()
	roots.AddCert(anchor)

	intermediates := x509.NewCertPool()
	for _, cert := range chain[1 : len(chain)-1] {
		intermediates.AddCert(cert)
	}

	for _, usage := range []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth} {
		if _, err := chain[0].Verify(x509.VerifyOptions{
			Roots:         roots,
			Intermediates: intermediates,
			CurrentTime:   now,
			KeyUsages:     []x509.ExtKeyUsage{usage},
		}); err != nil {
			return fmt.Errorf("the certificate does not chain to the last certificate of the chain: %w", err)
		}
	}

	return nil
}

func isCACert(cert *x509.Certificate) bool {
	return cert.BasicConstraintsValid && cert.IsCA && (cert.KeyUsage == 0 || cert.KeyUsage&x509.KeyUsageCertSign != 0)
}

func isSelfSigned(cert *x509.Certificate) bool {
	return bytes.Equal(cert.RawIssuer, cert.RawSubject) && cert.CheckSignature(cert.SignatureAlgorithm, cert.RawTBSCertificate, cert.Signature) == nil
}

func ValidateTrustAnchorPEM(data []byte, now time.Time) ([]*x509.Certificate, error) {
	certs, err := ParseCertChainPEM(data)
	if err != nil {
		return nil, err
	}

	for i, cert := range certs {
		if !isCACert(cert) && !isSelfSigned(cert) {
			return nil, fmt.Errorf("certificate %d must be a CA certificate (root or sub-CA) or a self-signed certificate", i+1)
		}

		if now.After(cert.NotAfter) {
			return nil, fmt.Errorf("certificate %d expired on %s", i+1, cert.NotAfter.UTC().Format(time.RFC3339))
		}

		if now.Before(cert.NotBefore) {
			return nil, fmt.Errorf("certificate %d is not valid before %s", i+1, cert.NotBefore.UTC().Format(time.RFC3339))
		}

		if err := checkAnchorKey(cert.PublicKey); err != nil {
			return nil, fmt.Errorf("certificate %d: %w", i+1, err)
		}
	}

	return certs, nil
}

func checkRSAKey(k *rsa.PublicKey) error {
	if k.N.BitLen() < 2048 {
		return fmt.Errorf("RSA key must be at least 2048 bits, got %d", k.N.BitLen())
	}

	if k.E < minRSAExponent {
		return fmt.Errorf("RSA public exponent must be at least %d, got %d", minRSAExponent, k.E)
	}

	return nil
}

func checkSEPPKey(pub crypto.PublicKey) error {
	switch k := pub.(type) {
	case *ecdsa.PublicKey:
		if k.Curve != elliptic.P256() && k.Curve != elliptic.P384() {
			return fmt.Errorf("ECDSA key must use P-256 or P-384, got %s", k.Curve.Params().Name)
		}
	case *rsa.PublicKey:
		return checkRSAKey(k)
	default:
		return fmt.Errorf("key must be ECDSA P-256, ECDSA P-384 or RSA of at least 2048 bits, got %T", pub)
	}

	return nil
}

func checkAnchorKey(pub crypto.PublicKey) error {
	switch k := pub.(type) {
	case *ecdsa.PublicKey:
		if k.Curve != elliptic.P256() && k.Curve != elliptic.P384() && k.Curve != elliptic.P521() {
			return fmt.Errorf("ECDSA key must use P-256, P-384 or P-521, got %s", k.Curve.Params().Name)
		}
	case *rsa.PublicKey:
		return checkRSAKey(k)
	default:
		return fmt.Errorf("key must be ECDSA or RSA of at least 2048 bits, got %T", pub)
	}

	return nil
}

func hasServerAndClientAuth(cert *x509.Certificate) bool {
	if len(cert.ExtKeyUsage) == 0 && len(cert.UnknownExtKeyUsage) == 0 {
		return true
	}

	return slices.Contains(cert.ExtKeyUsage, x509.ExtKeyUsageServerAuth) && slices.Contains(cert.ExtKeyUsage, x509.ExtKeyUsageClientAuth)
}

func seppFQDNOfLeaf(leaf *x509.Certificate, requested string, plmn PLMN) (string, error) {
	domain := plmn.Domain()

	if cn := leaf.Subject.CommonName; cn != "" {
		if name, err := ParseSEPPName(strings.ToLower(cn)); err == nil && name.WellKnown() {
			return "", fmt.Errorf("certificate subject CN %q is the well-known SEPP FQDN, which must not appear in a certificate (GSMA FS.34 clause 5.1.1)", cn)
		}
	}

	var candidates []string

	for _, raw := range leaf.DNSNames {
		named, ok := PLMNOfFQDN(raw)
		if !ok {
			continue
		}

		if !named.Equivalent(plmn) {
			return "", fmt.Errorf("certificate SAN %q names another PLMN than %s", raw, domain)
		}

		lower := strings.ToLower(raw)

		if strings.HasPrefix(lower, "*") {
			return "", fmt.Errorf("certificate SAN %q must not be a wildcard", raw)
		}

		if !strings.HasSuffix(lower, "."+domain) {
			continue
		}

		name, err := ParseSEPPName(lower)
		if err != nil {
			return "", fmt.Errorf("certificate SAN %q: %w", raw, err)
		}

		if name.WellKnown() {
			return "", fmt.Errorf("certificate SAN %q is the well-known SEPP FQDN, which must not appear in a certificate (GSMA FS.34 clause 5.1.1)", raw)
		}

		candidates = append(candidates, lower)
	}

	if len(candidates) == 0 {
		return "", fmt.Errorf("certificate must have a SAN DNS name under %s", domain)
	}

	if requested != "" {
		requested = strings.ToLower(requested)
		if !slices.Contains(candidates, requested) {
			return "", fmt.Errorf("fqdn %q is not one of the certificate's SAN DNS names under %s", requested, domain)
		}

		return requested, nil
	}

	if len(candidates) == 1 {
		return candidates[0], nil
	}

	if cn := strings.ToLower(leaf.Subject.CommonName); slices.Contains(candidates, cn) {
		return cn, nil
	}

	return "", fmt.Errorf("certificate has several SAN DNS names under %s (%s) and none is its subject CN: give the fqdn to use", domain, strings.Join(candidates, ", "))
}
