// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package asn1spec_test

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/ellanetworks/core/per/asn1spec"
)

func countMatches(t *testing.T, dir string, re *regexp.Regexp) int {
	t.Helper()

	files, err := filepath.Glob(filepath.Join(dir, "*.asn"))
	if err != nil {
		t.Fatal(err)
	}

	n := 0

	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}

		n += len(re.FindAll(b, -1))
	}

	return n
}

var (
	ieIDAssignment      = regexp.MustCompile(`(?m)^\s*id-[A-Za-z0-9-]+\s+ProtocolIE-ID\s*::=`)
	procCodeAssignment  = regexp.MustCompile(`(?m)^\s*id-[A-Za-z0-9-]+\s+ProcedureCode\s*::=`)
	objectSetAssignment = regexp.MustCompile(`(?m)^[A-Za-z0-9-]+\s+[A-Z0-9]+-(PROTOCOL-IES|PROTOCOL-IES-PAIR|PROTOCOL-EXTENSION|ELEMENTARY-PROCEDURE|PRIVATE-IES)\s*::=`)
)

func TestVendoredSpecs(t *testing.T) {
	tests := []struct {
		proto   string
		dir     string
		modules []string
	}{
		{"S1AP", "../../s1ap/testdata/asn1", []string{"S1AP-CommonDataTypes", "S1AP-Constants", "S1AP-Containers", "S1AP-IEs", "S1AP-PDU-Contents", "S1AP-PDU-Descriptions", "SonTransfer-IEs"}},
		{"NGAP", "../../ngap/testdata/asn1", []string{"NGAP-CommonDataTypes", "NGAP-Constants", "NGAP-Containers", "NGAP-IEs", "NGAP-PDU-Contents", "NGAP-PDU-Descriptions"}},
	}

	for _, tt := range tests {
		t.Run(tt.proto, func(t *testing.T) {
			s, err := asn1spec.ParseFS(os.DirFS(tt.dir), "*.asn")
			if err != nil {
				t.Fatal(err)
			}

			if len(s.Modules) != len(tt.modules) {
				t.Fatalf("got %d modules", len(s.Modules))
			}

			for i, name := range tt.modules {
				if s.Modules[i].Name != name {
					t.Errorf("module %d = %s, want %s", i, s.Modules[i].Name, name)
				}
			}

			var ieIDs, procCodes, objectSets int

			ids := map[int64]string{}
			codes := map[int64]string{}

			for _, m := range s.Modules {
				objectSets += len(m.ObjectSets) + len(m.Objects)

				for _, va := range m.Values {
					n, err := s.Int(va.Value)
					if err != nil {
						t.Errorf("%s: %v", va.Name, err)
					}

					switch va.Type.Name {
					case "ProtocolIE-ID":
						ieIDs++

						if prev, dup := ids[n]; dup {
							t.Errorf("ProtocolIE-ID %d assigned to %s and %s", n, prev, va.Name)
						}

						ids[n] = va.Name
					case "ProcedureCode":
						procCodes++

						if prev, dup := codes[n]; dup {
							t.Errorf("ProcedureCode %d assigned to %s and %s", n, prev, va.Name)
						}

						codes[n] = va.Name
					}
				}

				for _, sa := range m.ObjectSets {
					objs, err := s.Objects(sa.Set)
					if err != nil {
						t.Errorf("%s: %v", sa.Name, err)
					}

					for _, o := range objs {
						if o.Settings == nil {
							t.Errorf("%s: unresolved object at %s", sa.Name, o.Pos)
						}
					}
				}
			}

			if want := countMatches(t, tt.dir, ieIDAssignment); ieIDs != want {
				t.Errorf("parsed %d ProtocolIE-ID values, source has %d", ieIDs, want)
			}

			if want := countMatches(t, tt.dir, procCodeAssignment); procCodes != want {
				t.Errorf("parsed %d ProcedureCode values, source has %d", procCodes, want)
			}

			if want := countMatches(t, tt.dir, objectSetAssignment); objectSets != want {
				t.Errorf("parsed %d objects and object sets, source has %d", objectSets, want)
			}

			procs := s.ObjectSet(tt.proto + "-ELEMENTARY-PROCEDURES")
			if procs == nil {
				t.Fatal("no elementary procedure set")
			}

			objs, err := s.Objects(procs.Set)
			if err != nil {
				t.Fatal(err)
			}

			seen := map[int64]bool{}

			for _, o := range objs {
				v, ok := o.Value("&procedureCode")
				if !ok {
					t.Fatalf("procedure without code at %s", o.Pos)
				}

				n, err := s.Int(v)
				if err != nil {
					t.Fatal(err)
				}

				seen[n] = true

				if o.Type("&InitiatingMessage") == nil {
					t.Errorf("procedure %s has no initiating message", v.Ref)
				}
			}

			if len(seen) != len(objs) || len(objs) != procCodes {
				t.Errorf("%d procedures, %d distinct codes, %d code values", len(objs), len(seen), procCodes)
			}
		})
	}
}

func TestResolveScoped(t *testing.T) {
	s, err := asn1spec.ParseFS(os.DirFS("../../s1ap/testdata/asn1"), "*.asn")
	if err != nil {
		t.Fatal(err)
	}

	son := s.Module("SonTransfer-IEs")

	m, x := s.Resolve(son, "MobilityInformation")
	if m != son {
		t.Fatalf("MobilityInformation resolved in %v", m)
	}

	if ta, ok := x.(*asn1spec.TypeAssignment); !ok || ta != son.Type("MobilityInformation") {
		t.Errorf("resolved %v", x)
	}

	pc := s.Module("S1AP-PDU-Contents")

	m, _ = s.Resolve(pc, "ProtocolIE-Container")
	if m == nil || m.Name != "S1AP-Containers" {
		t.Errorf("ProtocolIE-Container resolved in %v", m)
	}
}
