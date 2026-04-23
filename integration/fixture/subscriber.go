package fixture

import (
	"strings"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/internal/tester/scenarios"
)

// SubscriberSpec describes one subscriber.
type SubscriberSpec struct {
	IMSI           string
	Key            string
	OPc            string
	SequenceNumber string
	ProfileName    string
}

// DefaultSubscriberSpec returns the scenarios-package default subscriber.
func DefaultSubscriberSpec() SubscriberSpec {
	return SubscriberSpec{
		IMSI:           scenarios.DefaultIMSI,
		Key:            scenarios.DefaultKey,
		OPc:            scenarios.DefaultOPC,
		SequenceNumber: scenarios.DefaultSequenceNumber,
		ProfileName:    scenarios.DefaultProfileName,
	}
}

// Subscriber creates a subscriber. If one with the same IMSI already
// exists, verify its profile matches; fail on mismatch.
//
// We do not verify Key/OPc because the subscribers list endpoint does
// not return them; changing Key/OPc across subtests would go silently
// undetected. All scenarios share scenarios.DefaultKey/OPC so this
// does not come up in practice.
func (f *F) Subscriber(spec SubscriberSpec) {
	f.t.Helper()

	existing, err := f.c.GetSubscriber(f.ctx, &client.GetSubscriberOptions{ID: spec.IMSI})
	if err == nil {
		if existing.ProfileName != spec.ProfileName {
			f.fatalf("subscriber %q exists with profile %q, want %q",
				spec.IMSI, existing.ProfileName, spec.ProfileName)
		}

		return
	}

	if !isNotFound(err) {
		f.fatalf("get subscriber %q: %v", spec.IMSI, err)
	}

	if err := f.c.CreateSubscriber(f.ctx, &client.CreateSubscriberOptions{
		Imsi:           spec.IMSI,
		Key:            spec.Key,
		SequenceNumber: spec.SequenceNumber,
		ProfileName:    spec.ProfileName,
		OPc:            spec.OPc,
	}); err != nil {
		f.fatalf("create subscriber %q: %v", spec.IMSI, err)
	}
}

// SubscriberBatch creates count subscribers whose IMSIs start at baseIMSI
// and increment by 1.
func (f *F) SubscriberBatch(baseIMSI string, count int, tmpl SubscriberSpec) {
	f.t.Helper()

	if len(baseIMSI) != 15 {
		f.fatalf("SubscriberBatch: baseIMSI %q must be 15 digits", baseIMSI)
	}

	for i := 0; i < count; i++ {
		spec := tmpl
		spec.IMSI = incrementIMSI(baseIMSI, i)

		f.Subscriber(spec)
	}
}

func incrementIMSI(base string, offset int) string {
	var n uint64

	for _, ch := range base {
		n = n*10 + uint64(ch-'0')
	}

	n += uint64(offset)

	out := make([]byte, 15)
	for i := 14; i >= 0; i-- {
		out[i] = byte('0' + n%10)
		n /= 10
	}

	return string(out)
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}

	msg := err.Error()

	return strings.Contains(msg, "not found") || strings.Contains(msg, "404")
}
