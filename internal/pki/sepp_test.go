// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package pki_test

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"math/big"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/pki"
)

var testPLMN = pki.PLMN{MCC: "001", MNC: "01"}

func TestPLMNDomain(t *testing.T) {
	cases := []struct {
		plmn pki.PLMN
		want string
	}{
		{pki.PLMN{MCC: "001", MNC: "01"}, "5gc.mnc001.mcc001.3gppnetwork.org"},
		{pki.PLMN{MCC: "208", MNC: "93"}, "5gc.mnc093.mcc208.3gppnetwork.org"},
		{pki.PLMN{MCC: "310", MNC: "410"}, "5gc.mnc410.mcc310.3gppnetwork.org"},
	}

	for _, tc := range cases {
		if got := tc.plmn.Domain(); got != tc.want {
			t.Errorf("Domain(%+v) = %q, want %q", tc.plmn, got, tc.want)
		}
	}

	if got := testPLMN.DefaultSEPPFQDN(); got != "sepp1.5gc.mnc001.mcc001.3gppnetwork.org" {
		t.Errorf("DefaultSEPPFQDN = %q", got)
	}

	if !(pki.PLMN{MCC: "001", MNC: "01"}).Equivalent(pki.PLMN{MCC: "001", MNC: "001"}) || (pki.PLMN{MCC: "001", MNC: "01"}).Equivalent(pki.PLMN{MCC: "001", MNC: "010"}) {
		t.Error("Equivalent does not compare zero-padded MNCs")
	}
}

func TestPLMNOfFQDN(t *testing.T) {
	cases := []struct {
		name string
		want pki.PLMN
		ok   bool
	}{
		{"sepp1.5gc.mnc093.mcc208.3gppnetwork.org", pki.PLMN{MCC: "208", MNC: "093"}, true},
		{"SEPP1.5GC.MNC093.MCC208.3GPPNETWORK.ORG", pki.PLMN{MCC: "208", MNC: "093"}, true},
		{"mme.epc.mnc001.mcc001.3gppnetwork.org", pki.PLMN{MCC: "001", MNC: "001"}, true},
		{"sepp1.5gc.mnc010.mcc207.ipxa.ipxnetwork.org", pki.PLMN{MCC: "207", MNC: "010"}, true},
		{"sepp1.5gc.mnc93.mcc208.3gppnetwork.org", pki.PLMN{}, false},
		{"sepp1.example.org", pki.PLMN{}, false},
		{"mnc001.mcc01.3gppnetwork.org", pki.PLMN{}, false},
		{"diameter.ipxa.ipxnetwork.org", pki.PLMN{}, false},
	}

	for _, tc := range cases {
		got, ok := pki.PLMNOfFQDN(tc.name)
		if ok != tc.ok || got != tc.want {
			t.Errorf("PLMNOfFQDN(%q) = %+v, %v; want %+v, %v", tc.name, got, ok, tc.want, tc.ok)
		}
	}
}

func TestValidateSEPPFQDN(t *testing.T) {
	valid := []string{
		"sepp1.5gc.mnc001.mcc001.3gppnetwork.org",
		"sepp-a2.5gc.mnc001.mcc001.3gppnetwork.org",
	}

	for _, fqdn := range valid {
		if err := pki.ValidateSEPPFQDN(fqdn, testPLMN); err != nil {
			t.Errorf("ValidateSEPPFQDN(%q) = %v, want nil", fqdn, err)
		}
	}

	invalid := []string{
		"",
		"5gc.mnc001.mcc001.3gppnetwork.org",
		"sepp.5gc.mnc001.mcc001.3gppnetwork.org",
		"edge.5gc.mnc001.mcc001.3gppnetwork.org",
		"sepp1.edge.5gc.mnc001.mcc001.3gppnetwork.org",
		"sepp1.5gc.mnc002.mcc001.3gppnetwork.org",
		"sepp1.epc.mnc001.mcc001.3gppnetwork.org",
		"sepp1.example.org",
		"SEPP1.5gc.mnc001.mcc001.3gppnetwork.org",
		"*.5gc.mnc001.mcc001.3gppnetwork.org",
		"-sepp.5gc.mnc001.mcc001.3gppnetwork.org",
		"se_pp1.5gc.mnc001.mcc001.3gppnetwork.org",
		"a..5gc.mnc001.mcc001.3gppnetwork.org",
		"sepp" + strings.Repeat("a", 64) + ".5gc.mnc001.mcc001.3gppnetwork.org",
	}

	for _, fqdn := range invalid {
		if err := pki.ValidateSEPPFQDN(fqdn, testPLMN); err == nil {
			t.Errorf("ValidateSEPPFQDN(%q) = nil, want an error", fqdn)
		}
	}
}

func TestValidatePeerSEPPFQDN(t *testing.T) {
	plmns := []pki.PLMN{{MCC: "207", MNC: "10"}, {MCC: "207", MNC: "11"}}

	valid := []string{
		"sepp1.5gc.mnc010.mcc207.3gppnetwork.org",
		"sepp2.5gc.mnc011.mcc207.3gppnetwork.org",
		"edge.sepp.5gc.mnc010.mcc207.3gppnetwork.org",
		"sepp1.5gc.mnc010.mcc207.ipxa.ipxnetwork.org",
	}

	for _, fqdn := range valid {
		if err := pki.ValidatePeerSEPPFQDN(fqdn, plmns); err != nil {
			t.Errorf("ValidatePeerSEPPFQDN(%q) = %v, want nil", fqdn, err)
		}
	}

	invalid := map[string]string{
		"sepp.5gc.mnc010.mcc207.3gppnetwork.org":     "well-known",
		"sepp.5gc.mnc010.mcc207.ipxa.ipxnetwork.org": "well-known",
		"sepp1.5gc.mnc012.mcc207.3gppnetwork.org":    "not one of the partner's",
		"sepp1.5gc.mnc001.mcc001.3gppnetwork.org":    "not one of the partner's",
		"sepp1.epc.mnc010.mcc207.3gppnetwork.org":    "5GC domain",
		"sepp1.example.org":                          "must be <host>",
		"sepp_1.5gc.mnc010.mcc207.3gppnetwork.org":   "invalid label",
		"sepp1.5gc.mnc10.mcc207.3gppnetwork.org":     "3-digit",
		"5gc.mnc010.mcc207.3gppnetwork.org":          "must be <host>",
		"Sepp1.5gc.mnc010.mcc207.3gppnetwork.org":    "lowercase",
		"sepp1.5gc.mnc010.mcc207.ipxnetwork.org":     "must be <host>",
	}

	for fqdn, want := range invalid {
		if err := pki.ValidatePeerSEPPFQDN(fqdn, plmns); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("ValidatePeerSEPPFQDN(%q) = %v, want an error containing %q", fqdn, err, want)
		}
	}
}

func TestGenerateSEPPCert_Profile(t *testing.T) {
	plmn := pki.PLMN{MCC: "208", MNC: "93"}
	domain := "5gc.mnc093.mcc208.3gppnetwork.org"
	fqdn := "sepp1." + domain

	leaf, key, ca, err := pki.GenerateSEPPCert(fqdn, plmn)
	if err != nil {
		t.Fatalf("GenerateSEPPCert: %v", err)
	}

	if leaf.Version != 3 {
		t.Errorf("version = %d, want 3", leaf.Version)
	}

	if leaf.SerialNumber.Sign() <= 0 || leaf.SerialNumber.BitLen() > 159 {
		t.Errorf("serial %s is not a positive integer of at most 20 octets", leaf.SerialNumber)
	}

	pub, ok := leaf.PublicKey.(*ecdsa.PublicKey)
	if !ok || pub.Curve != elliptic.P256() {
		t.Errorf("leaf key is %T, want ECDSA P-256", leaf.PublicKey)
	}

	if leaf.SignatureAlgorithm != x509.ECDSAWithSHA256 {
		t.Errorf("signature algorithm = %s, want ECDSAWithSHA256", leaf.SignatureAlgorithm)
	}

	if !slices.Equal(leaf.Subject.Organization, []string{domain}) || leaf.Subject.CommonName != fqdn {
		t.Errorf("subject = %s, want O=%s, CN=%s", leaf.Subject, domain, fqdn)
	}

	for _, cert := range []*x509.Certificate{leaf, ca} {
		for _, tags := range subjectTags(t, cert) {
			if tags != asn1.TagUTF8String {
				t.Errorf("subject %s has an attribute encoded with tag %d, want UTF8String", cert.Subject, tags)
			}
		}
	}

	if got := leaf.NotBefore.AddDate(3, 0, 0); !leaf.NotAfter.Equal(got) {
		t.Errorf("validity %s - %s is not 3 years", leaf.NotBefore, leaf.NotAfter)
	}

	if leaf.NotBefore.After(time.Now()) {
		t.Errorf("notBefore %s is in the future", leaf.NotBefore)
	}

	if leaf.IsCA {
		t.Error("leaf is a CA")
	}

	if err := leaf.CheckSignatureFrom(ca); err != nil {
		t.Errorf("leaf is not signed by the CA: %v", err)
	}

	if leaf.Issuer.String() != ca.Subject.String() {
		t.Errorf("issuer = %s, want %s", leaf.Issuer, ca.Subject)
	}

	if !slices.Equal(leaf.DNSNames, []string{fqdn}) || len(leaf.IPAddresses) != 0 || len(leaf.URIs) != 0 || len(leaf.EmailAddresses) != 0 {
		t.Errorf("SAN = DNS %v, IP %v, URI %v, email %v; want only DNS %s", leaf.DNSNames, leaf.IPAddresses, leaf.URIs, leaf.EmailAddresses, fqdn)
	}

	if leaf.KeyUsage != x509.KeyUsageDigitalSignature {
		t.Errorf("keyUsage = %b, want digitalSignature", leaf.KeyUsage)
	}

	if !slices.Equal(leaf.ExtKeyUsage, []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}) || len(leaf.UnknownExtKeyUsage) != 0 {
		t.Errorf("extKeyUsage = %v, want serverAuth and clientAuth", leaf.ExtKeyUsage)
	}

	if !slices.Equal(leaf.AuthorityKeyId, ca.SubjectKeyId) || len(ca.SubjectKeyId) == 0 {
		t.Errorf("AKI = %x, want the CA SKI %x", leaf.AuthorityKeyId, ca.SubjectKeyId)
	}

	if want := method1KeyID(t, leaf.PublicKey); !slices.Equal(leaf.SubjectKeyId, want) {
		t.Errorf("SKI = %x, want %x", leaf.SubjectKeyId, want)
	}

	if want := method1KeyID(t, ca.PublicKey); !slices.Equal(ca.SubjectKeyId, want) {
		t.Errorf("CA SKI = %x, want %x", ca.SubjectKeyId, want)
	}

	if len(leaf.CRLDistributionPoints) != 0 {
		t.Errorf("CRL distribution points = %v, want none", leaf.CRLDistributionPoints)
	}

	criticality := map[string]bool{}
	for _, ext := range leaf.Extensions {
		criticality[ext.Id.String()] = ext.Critical
	}

	wantExts := map[string]bool{
		"2.5.29.15":          true,
		"2.5.29.37":          false,
		"2.5.29.35":          false,
		"2.5.29.14":          false,
		"2.5.29.17":          true,
		"1.3.6.1.5.5.7.1.34": false,
	}

	for oid, critical := range wantExts {
		got, ok := criticality[oid]
		if !ok {
			t.Errorf("extension %s is missing", oid)
			continue
		}

		if got != critical {
			t.Errorf("extension %s critical = %v, want %v", oid, got, critical)
		}
	}

	if len(criticality) != len(wantExts) {
		t.Errorf("extensions = %v, want exactly %v", criticality, wantExts)
	}

	for _, ext := range leaf.Extensions {
		if !ext.Id.Equal(pki.OIDExtensionNFTypes) {
			continue
		}

		var nfTypes []asn1.RawValue

		if rest, err := asn1.Unmarshal(ext.Value, &nfTypes); err != nil || len(rest) != 0 {
			t.Fatalf("decode nfTypes: %v", err)
		}

		if len(nfTypes) != 1 || nfTypes[0].Tag != asn1.TagIA5String || string(nfTypes[0].Bytes) != "SEPP" {
			t.Errorf("nfTypes = %+v, want [IA5String SEPP]", nfTypes)
		}
	}

	if !ca.IsCA || !ca.BasicConstraintsValid || ca.MaxPathLen != 0 || !ca.MaxPathLenZero {
		t.Errorf("CA basic constraints: IsCA %v, MaxPathLen %d, MaxPathLenZero %v", ca.IsCA, ca.MaxPathLen, ca.MaxPathLenZero)
	}

	if ca.KeyUsage != x509.KeyUsageCertSign|x509.KeyUsageCRLSign {
		t.Errorf("CA keyUsage = %b, want keyCertSign|cRLSign", ca.KeyUsage)
	}

	if caPub, ok := ca.PublicKey.(*ecdsa.PublicKey); !ok || caPub.Curve != elliptic.P256() {
		t.Errorf("CA key is %T, want ECDSA P-256", ca.PublicKey)
	}

	if err := ca.CheckSignatureFrom(ca); err != nil {
		t.Errorf("CA is not self-signed: %v", err)
	}

	roots := x509.NewCertPool()
	roots.AddCert(ca)

	for _, usage := range []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth} {
		if _, err := leaf.Verify(x509.VerifyOptions{Roots: roots, DNSName: fqdn, KeyUsages: []x509.ExtKeyUsage{usage}}); err != nil {
			t.Errorf("verify leaf for %v: %v", usage, err)
		}
	}

	keyPEM, err := pki.EncodePrivateKeyPEM(key)
	if err != nil {
		t.Fatalf("encode key: %v", err)
	}

	chainPEM := string(pki.EncodeCertPEM(leaf)) + string(pki.EncodeCertPEM(ca))

	cert, err := pki.ValidateSEPPCertificate([]byte(chainPEM), keyPEM, "", plmn, time.Now())
	if err != nil {
		t.Fatalf("ValidateSEPPCertificate of a generated identity: %v", err)
	}

	if cert.FQDN != fqdn || !cert.TrustAnchor().Equal(ca) || !cert.Leaf().Equal(leaf) {
		t.Errorf("validated certificate = %+v", cert)
	}

	if warnings := cert.ProfileWarnings(); !slices.Equal(warnings, []string{pki.SEPPWarningNoCRLDistributor}) {
		t.Errorf("ProfileWarnings = %q, want only the missing CRL distribution point", warnings)
	}

	if cert.ChainPEM() != chainPEM {
		t.Error("ChainPEM does not round-trip")
	}
}

func TestGenerateSEPPCert_RejectsFQDNOutsideDomain(t *testing.T) {
	if _, _, _, err := pki.GenerateSEPPCert("sepp1.5gc.mnc002.mcc001.3gppnetwork.org", testPLMN); err == nil {
		t.Fatal("GenerateSEPPCert accepted an FQDN of another PLMN")
	}

	if _, _, _, err := pki.GenerateSEPPCert("sepp.5gc.mnc001.mcc001.3gppnetwork.org", testPLMN); err == nil {
		t.Fatal("GenerateSEPPCert accepted the well-known SEPP FQDN")
	}
}

func TestGenerateSEPPCert_FreshCAEachTime(t *testing.T) {
	_, _, ca1, err := pki.GenerateSEPPCert(testPLMN.DefaultSEPPFQDN(), testPLMN)
	if err != nil {
		t.Fatal(err)
	}

	_, _, ca2, err := pki.GenerateSEPPCert(testPLMN.DefaultSEPPFQDN(), testPLMN)
	if err != nil {
		t.Fatal(err)
	}

	if ca1.Equal(ca2) {
		t.Fatal("two generations share a CA")
	}
}

func subjectTags(t *testing.T, cert *x509.Certificate) []int {
	t.Helper()

	var rdns pkix.RDNSequence
	if _, err := asn1.Unmarshal(cert.RawSubject, &rdns); err != nil {
		t.Fatal(err)
	}

	var raw []asn1.RawValue
	if _, err := asn1.Unmarshal(cert.RawSubject, &raw); err != nil {
		t.Fatal(err)
	}

	var tags []int

	for _, set := range raw {
		var attrs []struct {
			Type  asn1.ObjectIdentifier
			Value asn1.RawValue
		}

		if _, err := asn1.UnmarshalWithParams(set.FullBytes, &attrs, "set"); err != nil {
			t.Fatal(err)
		}

		for _, a := range attrs {
			tags = append(tags, a.Value.Tag)
		}
	}

	if len(tags) != 2 || len(rdns) != 2 {
		t.Fatalf("subject %s has %d attributes, want O and CN", cert.Subject, len(tags))
	}

	return tags
}

func method1KeyID(t *testing.T, pub crypto.PublicKey) []byte {
	t.Helper()

	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}

	var spki struct {
		Algorithm pkix.AlgorithmIdentifier
		PublicKey asn1.BitString
	}

	if _, err := asn1.Unmarshal(der, &spki); err != nil {
		t.Fatal(err)
	}

	sum := sha1.Sum(spki.PublicKey.Bytes)

	return sum[:]
}

type testCA struct {
	cert *x509.Certificate
	key  crypto.Signer
}

func newTestCA(t *testing.T) testCA {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}

	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}

	return testCA{cert: cert, key: key}
}

func (ca testCA) issue(t *testing.T, key crypto.Signer, mutate func(*x509.Certificate)) *x509.Certificate {
	t.Helper()

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: testPLMN.DefaultSEPPFQDN()},
		DNSNames:     []string{testPLMN.DefaultSEPPFQDN()},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().AddDate(1, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}

	if mutate != nil {
		mutate(tmpl)
	}

	parent, signer := ca.cert, ca.key
	if parent == nil {
		parent, signer = tmpl, key
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, parent, key.Public(), signer)
	if err != nil {
		t.Fatal(err)
	}

	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}

	return cert
}

func newP256(t *testing.T) crypto.Signer {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	return key
}

func keyPEM(t *testing.T, key crypto.Signer) []byte {
	t.Helper()

	out, err := pki.EncodePrivateKeyPEM(key)
	if err != nil {
		t.Fatal(err)
	}

	return out
}

func chainPEM(certs ...*x509.Certificate) []byte {
	var out []byte
	for _, c := range certs {
		out = append(out, pki.EncodeCertPEM(c)...)
	}

	return out
}

func (ca testCA) subCA(t *testing.T, mutate func(*x509.Certificate)) testCA {
	t.Helper()

	key := newP256(t)
	cert := ca.issue(t, key, func(c *x509.Certificate) {
		c.Subject = pkix.Name{CommonName: "test sub-CA"}
		c.DNSNames = nil
		c.ExtKeyUsage = nil
		c.KeyUsage = x509.KeyUsageCertSign
		c.BasicConstraintsValid = true
		c.IsCA = true
		c.SerialNumber = big.NewInt(3)

		if mutate != nil {
			mutate(c)
		}
	})

	return testCA{cert: cert, key: key}
}

func TestValidateSEPPCertificate(t *testing.T) {
	ca := newTestCA(t)
	otherCA := newTestCA(t)
	sub := ca.subCA(t, nil)
	expiredSub := ca.subCA(t, func(c *x509.Certificate) {
		c.NotBefore = time.Now().AddDate(-2, 0, 0)
		c.NotAfter = time.Now().Add(-time.Hour)
	})
	key := newP256(t)
	fqdn := testPLMN.DefaultSEPPFQDN()
	sepp2 := "sepp2.5gc.mnc001.mcc001.3gppnetwork.org"

	rsa2048, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	rsa1024, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatal(err)
	}

	p384, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	p224, err := ecdsa.GenerateKey(elliptic.P224(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	_, edKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	leaf := func(k crypto.Signer, mutate func(*x509.Certificate)) *x509.Certificate {
		return ca.issue(t, k, mutate)
	}

	selfSigned := testCA{}.issue(t, key, nil)
	notCAKey := newP256(t)
	notCA := ca.issue(t, notCAKey, nil)
	leafOfNotCA := testCA{cert: notCA, key: notCAKey}.issue(t, key, nil)

	cases := []struct {
		name      string
		chain     []byte
		key       []byte
		fqdn      string
		wantErr   string
		wantFQDN  string
		wantChain int
		wantWarn  []string
	}{
		{name: "leaf and CA", chain: chainPEM(leaf(key, nil), ca.cert), key: keyPEM(t, key), wantChain: 2, wantWarn: []string{pki.SEPPWarningNoNFTypes, pki.SEPPWarningNoCRLDistributor}},
		{name: "leaf, sub-CA and root", chain: chainPEM(sub.issue(t, key, nil), sub.cert, ca.cert), key: keyPEM(t, key), wantChain: 3},
		{name: "sub-CA anchor", chain: chainPEM(sub.issue(t, key, nil), sub.cert), key: keyPEM(t, key), wantChain: 2},
		{name: "P-384", chain: chainPEM(leaf(p384, nil), ca.cert), key: keyPEM(t, p384), wantChain: 2},
		{name: "RSA 2048", chain: chainPEM(leaf(rsa2048, nil), ca.cert), key: keyPEM(t, rsa2048), wantChain: 2},
		{name: "no EKU warns", chain: chainPEM(leaf(key, func(c *x509.Certificate) { c.ExtKeyUsage = nil }), ca.cert), key: keyPEM(t, key), wantChain: 2, wantWarn: []string{pki.SEPPWarningNoEKU, pki.SEPPWarningNoNFTypes, pki.SEPPWarningNoCRLDistributor}},
		{name: "CN not in SAN warns", chain: chainPEM(leaf(key, func(c *x509.Certificate) { c.Subject.CommonName = "sepp" }), ca.cert), key: keyPEM(t, key), wantChain: 2, wantWarn: []string{pki.SEPPWarningNoNFTypes, pki.SEPPWarningCNNotInSAN, pki.SEPPWarningNoCRLDistributor}},
		{name: "host without SEPP ID warns", chain: chainPEM(leaf(key, func(c *x509.Certificate) {
			c.Subject.CommonName = "edge.5gc.mnc001.mcc001.3gppnetwork.org"
			c.DNSNames = []string{c.Subject.CommonName}
		}), ca.cert), key: keyPEM(t, key), wantChain: 2, wantFQDN: "edge.5gc.mnc001.mcc001.3gppnetwork.org", wantWarn: []string{pki.SEPPWarningNoNFTypes, pki.SEPPWarningHostNotSEPPID, pki.SEPPWarningNoCRLDistributor}},
		{name: "validity over 3 years warns", chain: chainPEM(leaf(key, func(c *x509.Certificate) { c.NotAfter = c.NotBefore.AddDate(5, 0, 0) }), ca.cert), key: keyPEM(t, key), wantChain: 2, wantWarn: []string{pki.SEPPWarningLongValidity, pki.SEPPWarningNoNFTypes, pki.SEPPWarningNoCRLDistributor}},
		{name: "extra non-3GPP SAN", chain: chainPEM(leaf(key, func(c *x509.Certificate) { c.DNSNames = []string{fqdn, "sepp.example.org"} }), ca.cert), key: keyPEM(t, key), wantChain: 2},
		{name: "uppercase SAN", chain: chainPEM(leaf(key, func(c *x509.Certificate) { c.DNSNames = []string{strings.ToUpper(fqdn)} }), ca.cert), key: keyPEM(t, key), wantChain: 2},
		{name: "two SANs, CN selects", chain: chainPEM(leaf(key, func(c *x509.Certificate) { c.DNSNames = []string{sepp2, fqdn} }), ca.cert), key: keyPEM(t, key), wantChain: 2},
		{name: "two SANs, fqdn selects", chain: chainPEM(leaf(key, func(c *x509.Certificate) { c.DNSNames = []string{fqdn, sepp2} }), ca.cert), key: keyPEM(t, key), fqdn: sepp2, wantFQDN: sepp2, wantChain: 2},
		{name: "two SANs, no CN among them", chain: chainPEM(leaf(key, func(c *x509.Certificate) {
			c.Subject.CommonName = ""
			c.DNSNames = []string{fqdn, sepp2}
		}), ca.cert), key: keyPEM(t, key), wantErr: "give the fqdn"},
		{name: "fqdn not in SAN", chain: chainPEM(leaf(key, nil), ca.cert), key: keyPEM(t, key), fqdn: sepp2, wantErr: "not one of the certificate's SAN"},
		{name: "well-known SAN", chain: chainPEM(leaf(key, func(c *x509.Certificate) {
			c.DNSNames = []string{fqdn, "sepp.5gc.mnc001.mcc001.3gppnetwork.org"}
		}), ca.cert), key: keyPEM(t, key), wantErr: "well-known"},
		{name: "well-known CN", chain: chainPEM(leaf(key, func(c *x509.Certificate) {
			c.Subject.CommonName = "sepp.5gc.mnc001.mcc001.3gppnetwork.org"
		}), ca.cert), key: keyPEM(t, key), wantErr: "well-known"},
		{name: "leaf only", chain: chainPEM(leaf(key, nil)), key: keyPEM(t, key), wantErr: "must end with the CA"},
		{name: "self-signed leaf", chain: chainPEM(selfSigned), key: keyPEM(t, key), wantErr: "must end with the CA"},
		{name: "last certificate is not a CA", chain: chainPEM(leafOfNotCA, notCA), key: keyPEM(t, key), wantErr: "must be a CA certificate"},
		{name: "leaf is a CA", chain: chainPEM(leaf(key, func(c *x509.Certificate) {
			c.BasicConstraintsValid = true
			c.IsCA = true
			c.KeyUsage |= x509.KeyUsageCertSign
		}), ca.cert), key: keyPEM(t, key), wantErr: "end-entity"},
		{name: "leaf not yet valid", chain: chainPEM(leaf(key, func(c *x509.Certificate) { c.NotBefore = time.Now().AddDate(0, 0, 10) }), ca.cert), key: keyPEM(t, key), wantErr: "does not chain"},
		{name: "expired sub-CA", chain: chainPEM(expiredSub.issue(t, key, nil), expiredSub.cert, ca.cert), key: keyPEM(t, key), wantErr: "does not chain"},
		{name: "unhandled critical extension", chain: chainPEM(leaf(key, func(c *x509.Certificate) {
			c.ExtraExtensions = []pkix.Extension{{Id: asn1.ObjectIdentifier{1, 2, 3, 4}, Critical: true, Value: []byte{0x05, 0x00}}}
		}), ca.cert), key: keyPEM(t, key), wantErr: "does not chain"},
		{name: "key mismatch", chain: chainPEM(leaf(key, nil), ca.cert), key: keyPEM(t, newP256(t)), wantErr: "key pair"},
		{name: "no key", chain: chainPEM(leaf(key, nil), ca.cert), key: nil, wantErr: "key pair"},
		{name: "P-224", chain: chainPEM(leaf(p224, nil), ca.cert), key: keyPEM(t, p224), wantErr: "P-256 or P-384"},
		{name: "RSA 1024", chain: chainPEM(leaf(rsa1024, nil), ca.cert), key: keyPEM(t, rsa1024), wantErr: "at least 2048"},
		{name: "Ed25519", chain: chainPEM(leaf(edKey, nil), ca.cert), key: keyPEM(t, edKey), wantErr: "key must be"},
		{name: "no digitalSignature", chain: chainPEM(leaf(key, func(c *x509.Certificate) { c.KeyUsage = x509.KeyUsageKeyAgreement }), ca.cert), key: keyPEM(t, key), wantErr: "digitalSignature"},
		{name: "serverAuth only", chain: chainPEM(leaf(key, func(c *x509.Certificate) { c.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth} }), ca.cert), key: keyPEM(t, key), wantErr: "extendedKeyUsage"},
		{name: "expired", chain: chainPEM(leaf(key, func(c *x509.Certificate) {
			c.NotBefore = time.Now().AddDate(-2, 0, 0)
			c.NotAfter = time.Now().Add(-time.Hour)
		}), ca.cert), key: keyPEM(t, key), wantErr: "expired"},
		{name: "no SAN", chain: chainPEM(leaf(key, func(c *x509.Certificate) { c.DNSNames = nil }), ca.cert), key: keyPEM(t, key), wantErr: "SAN DNS name under"},
		{name: "SAN of another PLMN", chain: chainPEM(leaf(key, func(c *x509.Certificate) { c.DNSNames = []string{"sepp1.5gc.mnc002.mcc001.3gppnetwork.org"} }), ca.cert), key: keyPEM(t, key), wantErr: "another PLMN"},
		{name: "wildcard SAN", chain: chainPEM(leaf(key, func(c *x509.Certificate) { c.DNSNames = []string{"*.5gc.mnc001.mcc001.3gppnetwork.org"} }), ca.cert), key: keyPEM(t, key), wantErr: "wildcard"},
		{name: "SAN is the domain itself", chain: chainPEM(leaf(key, func(c *x509.Certificate) { c.DNSNames = []string{"5gc.mnc001.mcc001.3gppnetwork.org"} }), ca.cert), key: keyPEM(t, key), wantErr: "SAN DNS name under"},
		{name: "wrong issuer in chain", chain: chainPEM(leaf(key, nil), otherCA.cert), key: keyPEM(t, key), wantErr: "does not chain"},
		{name: "chain out of order", chain: chainPEM(ca.cert, leaf(key, nil)), key: keyPEM(t, key), wantErr: "key pair"},
		{name: "non-certificate block", chain: append(chainPEM(leaf(key, nil), ca.cert), keyPEM(t, key)...), key: keyPEM(t, key), wantErr: "unexpected PEM block"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cert, err := pki.ValidateSEPPCertificate(tc.chain, tc.key, tc.fqdn, testPLMN, time.Now())
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want one containing %q", err, tc.wantErr)
				}

				return
			}

			if err != nil {
				t.Fatalf("err = %v, want nil", err)
			}

			wantFQDN := tc.wantFQDN
			if wantFQDN == "" {
				wantFQDN = fqdn
			}

			if cert.FQDN != wantFQDN {
				t.Errorf("FQDN = %q, want %q", cert.FQDN, wantFQDN)
			}

			if len(cert.Chain) != tc.wantChain {
				t.Errorf("chain length = %d, want %d", len(cert.Chain), tc.wantChain)
			}

			if tc.wantWarn != nil && !slices.Equal(cert.ProfileWarnings(), tc.wantWarn) {
				t.Errorf("ProfileWarnings = %q, want %q", cert.ProfileWarnings(), tc.wantWarn)
			}

			stored, err := pki.ValidateSEPPIdentity(tc.chain, tc.key, cert.FQDN)
			if err != nil || stored.FQDN != cert.FQDN || !stored.TrustAnchor().Equal(cert.TrustAnchor()) {
				t.Errorf("ValidateSEPPIdentity of the stored identity = %+v, %v", stored, err)
			}
		})
	}
}

func TestValidateTrustAnchorPEM(t *testing.T) {
	ca := newTestCA(t)
	sub := ca.subCA(t, nil)
	key := newP256(t)
	selfSigned := testCA{}.issue(t, key, nil)
	issuedLeaf := ca.issue(t, newP256(t), nil)
	future := ca.subCA(t, func(c *x509.Certificate) { c.NotBefore = time.Now().AddDate(0, 0, 1) })
	expired := ca.subCA(t, func(c *x509.Certificate) {
		c.NotBefore = time.Now().AddDate(-2, 0, 0)
		c.NotAfter = time.Now().Add(-time.Hour)
	})

	rsa1024, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatal(err)
	}

	weak := testCA{}.issue(t, rsa1024, func(c *x509.Certificate) {
		c.BasicConstraintsValid = true
		c.IsCA = true
		c.KeyUsage = x509.KeyUsageCertSign
	})

	rsa2048, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	smallExponent := rsa2048.PublicKey
	smallExponent.E = 3

	lowE, err := x509.CreateCertificate(rand.Reader, &x509.Certificate{
		SerialNumber:          big.NewInt(9),
		Subject:               pkix.Name{CommonName: "low exponent CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(1, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}, ca.cert, &smallExponent, ca.key)
	if err != nil {
		t.Fatal(err)
	}

	lowECert, err := x509.ParseCertificate(lowE)
	if err != nil {
		t.Fatal(err)
	}

	valid := map[string][]byte{
		"root":               chainPEM(ca.cert),
		"sub-CA":             chainPEM(sub.cert),
		"rollover bundle":    chainPEM(ca.cert, sub.cert),
		"self-signed pinned": chainPEM(selfSigned),
	}

	for name, data := range valid {
		if _, err := pki.ValidateTrustAnchorPEM(data, time.Now()); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}

	invalid := map[string]struct {
		data []byte
		want string
	}{
		"CA-issued leaf":  {chainPEM(issuedLeaf), "CA certificate"},
		"not yet valid":   {chainPEM(future.cert), "not valid before"},
		"expired":         {chainPEM(ca.cert, expired.cert), "certificate 2 expired"},
		"RSA 1024":        {chainPEM(weak), "at least 2048"},
		"RSA exponent 3":  {chainPEM(lowECert), "exponent"},
		"garbage":         {[]byte("not a certificate"), "no PEM certificate"},
		"private key PEM": {keyPEM(t, key), "unexpected PEM block"},
	}

	for name, tc := range invalid {
		if _, err := pki.ValidateTrustAnchorPEM(tc.data, time.Now()); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want one containing %q", name, err, tc.want)
		}
	}
}

func TestParseSEPPCertificate(t *testing.T) {
	leaf, _, ca, err := pki.GenerateSEPPCert(testPLMN.DefaultSEPPFQDN(), testPLMN)
	if err != nil {
		t.Fatal(err)
	}

	cert, err := pki.ParseSEPPCertificate(chainPEM(leaf, ca), testPLMN.DefaultSEPPFQDN())
	if err != nil {
		t.Fatal(err)
	}

	if cert.FQDN != testPLMN.DefaultSEPPFQDN() || !cert.TrustAnchor().Equal(ca) {
		t.Errorf("ParseSEPPCertificate = %+v", cert)
	}

	if _, err := pki.ParseSEPPCertificate(chainPEM(leaf, ca), "sepp2.5gc.mnc001.mcc001.3gppnetwork.org"); err == nil {
		t.Error("ParseSEPPCertificate accepted an FQDN that is not in the SAN")
	}
}
