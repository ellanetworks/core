package s1ap

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/per"
)

func TestZZProbe(t *testing.T) {
	for _, c := range loadCorpus(t) {
		pdu, _ := Unmarshal(c.b)
		codec, ok := typedCodecs[typedKey{TriggeringMessage(pdu.choiceIndex()), pdu.procedureCode()}]
		if !ok {
			continue
		}
		msg, err := codec(pdu.value())
		if err != nil {
			t.Logf("%s REFUSED %v", c.source, err)
			continue
		}
		re, err := msg.Marshal()
		if err != nil || !bytes.Equal(re, c.b) {
			t.Logf("%s NOTEXACT %v %T", c.source, err, msg)
			if err == nil {
				ra := per.NewReader(pdu.value()); _, _ = ra.ReadBit(); a, _ := decodeIEContainer(ra, per.Aligned)
				p2, _ := Unmarshal(re)
				rb := per.NewReader(p2.value()); _, _ = rb.ReadBit(); b, _ := decodeIEContainer(rb, per.Aligned)
				var ia, ib []ProtocolIEID
				for _, f := range a { ia = append(ia, f.id) }
				for _, f := range b { ib = append(ib, f.id) }
				t.Logf("   orig %x\n   re   %x", c.b, re)
				t.Logf("   wire %v\n   re   %v", ia, ib)
				for i := range a { for j := range b { if a[i].id == b[j].id && !bytes.Equal(a[i].value, b[j].value) { t.Logf("   IE %v differs\n    %x\n    %x", a[i].id, a[i].value, b[j].value) } } }
			}
		}
	}
}
