package fixture

import (
	"fmt"

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

// Subscriber creates a subscriber. Idempotent.
func (f *F) Subscriber(spec SubscriberSpec) {
	f.t.Helper()

	err := f.c.CreateSubscriber(f.ctx, &client.CreateSubscriberOptions{
		Imsi:           spec.IMSI,
		Key:            spec.Key,
		SequenceNumber: spec.SequenceNumber,
		ProfileName:    spec.ProfileName,
		OPc:            spec.OPc,
	})
	if err != nil && !isAlreadyExists(err) {
		f.fatalf("create subscriber %q: %v", spec.IMSI, err)
	}
}

// SubscriberBatch creates count subscribers whose IMSIs start at baseIMSI
// and increment by 1. Expects baseIMSI to be a 15-digit decimal string.
func (f *F) SubscriberBatch(baseIMSI string, count int, tmpl SubscriberSpec) {
	f.t.Helper()

	base := baseIMSI

	if len(base) != 15 {
		f.fatalf("SubscriberBatch: baseIMSI %q must be 15 digits", base)
	}

	for i := 0; i < count; i++ {
		imsi := incrementIMSI(base, i)
		spec := tmpl
		spec.IMSI = imsi

		f.Subscriber(spec)
	}
}

func incrementIMSI(base string, offset int) string {
	var n uint64

	for _, ch := range base {
		n = n*10 + uint64(ch-'0')
	}

	n += uint64(offset)

	return fmt.Sprintf("%015d", n)
}
