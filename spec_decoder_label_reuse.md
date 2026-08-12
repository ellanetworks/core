# Spec: Reuse Codec Vocabulary in `internal/decoder`

## Goal

`internal/decoder` maintains its own copies of protocol vocabulary — IE names, procedure names, IE identifiers, criticality, and enum labels — that the `ngap`, `s1ap` and `nas` libraries already define. Make the codec libraries the single definition and have the decoder consume them.

The authority for a name is the TS ASN.1 identifier, not whichever table already holds it. Measured against TS 38.413 and TS 36.413, both sides are wrong in places: the codec de-hyphenates 8 NGAP ids and 59 S1AP ids, while the decoder de-hyphenates 7 NGAP ids. Step 2 reconciles both against the spec, so IE labels change in the UI and in `ProtocolIEID.String()` error text.

## Current state

| Data | Codec (authoritative) | Decoder copy |
|---|---|---|
| NGAP IE names | `ngap/ie_names.go` `protocolIENames`, 98 entries | `internal/decoder/ngap/util.go` `ieNames`, 158 entries, keyed by integer literal |
| NGAP procedure names | `ngap/procedure_names.go`, 81 entries | `internal/decoder/ngap/util.go`, 81 entries, spelling already identical |
| S1AP IE names | `s1ap/ie_names.go`, 90 entries | `internal/decoder/s1ap/util.go`, 42 entries |
| S1AP procedure names | `s1ap/procedure_names.go`, 67 entries | `internal/decoder/s1ap/util.go`, 67 entries, spelling already identical |
| IE identifiers | `ngap/ie_ids.go`, `s1ap/ie_ids.go`, unexported | 46 + 43 re-declared constants in each decoder's `common_ie.go` / `util.go` |
| NAS enum labels | `nas/fgs`, `nas/eps` — 90 `String()` methods over `enumString` | 179 `utils.MakeEnum` sites across `internal/decoder/nas`, `internal/decoder/eps` |
| Cause names | `ngap/cause_names.go`, `s1ap/cause_names.go` via `ValueName()` | already consumed — this is the pattern to follow |

## Step 0 — Fix the wrong S1AP identifier

`internal/decoder/s1ap` declares `idUERadioCapabilityForPaging = 117`; TS 36.413 assigns 198, and 117 is `WarningSecurityInfo`. Correct it with a test pinning the emitted IE id, before the refactor makes it invisible.

## Step 1 — Add bare-name accessors to `ngap` and `s1ap`

In each module, alongside the existing tables:

```go
// ProtocolIEIDName returns the TS 38.413 name of id, and whether it is known.
func ProtocolIEIDName(id ProtocolIEID) (string, bool) {
    name, ok := protocolIENames[id]
    return name, ok
}
```

Same shape for `ProcedureCodeName`. Reimplement `String()` on top so there is one lookup path.

## Step 2 — Reconcile both tables against the spec

Add the 63 ids the decoder names and the codec does not, with their TS 38.413 identifiers and constants. Correct the codec entries that deviate from the ASN.1 spelling: NGAP ids 10, 26, 38, 85, 100, 111, 114, 167, and 59 S1AP entries — including two outright errors, id 48 (`E-RABFailedToSetupListCtxtSURes`) and id 75 (`GUMMEI-ID`).

Land this on its own, with a test asserting every id the decoder previously named is still named. `s1ap/names_test.go` moves with it.

## Step 3 — Delete the decoder's name tables

Replace `ieNames` / `procedureNames` lookups in `internal/decoder/ngap/util.go` and `internal/decoder/s1ap/util.go` with:

```go
name, ok := ngap.ProtocolIEIDName(id)
return utils.MakeEnum(uint16(id), name, !ok)
```

Both procedure tables are already byte-identical to the codec's, so those two are pure deletes.

## Step 4 — Delete the re-declared IE identifier constants

Export the identifiers in `ngap/ie_ids.go` and `s1ap/ie_ids.go` — renamed, not aliased, so one spelling of each constant exists. Switch the decoder's `ie(...)` call sites to them and delete both decoder constant blocks.

## Step 5 — Route NAS enum labels through the codec

For each `utils.MakeEnum` site in `internal/decoder/nas` and `internal/decoder/eps` whose type has a `String()` in `nas/fgs` or `nas/eps`, delete the decoder-local table and take the codec's label. Where no `String()` exists, add one using the `enumString(v, map)` helper (`nas/fgs/enums.go`). One commit per enum type.

Known/unknown comes from a `(string, bool)` accessor per type, not from sniffing the label: `SecurityHeaderType`, `MessageType` and the cause types each render an unknown value differently.

The codec's wording stands, so labels change — `"RegistrationRequest"` → `"REGISTRATION REQUEST"`, `"Plain NAS"` → `"plain"`. Update the `"Plain NAS"` fallback in `ui/src/components/NGAPMessageRender.tsx` to match.

The NGAP and S1AP value enums go the same way — criticality, triggering message, type of error, cause group, paging DRX, time to wait, RRC establishment cause, handover type, CN domain, eNB ID kind — since the Goal counts them as vocabulary too. `internal/decoder/lpp` and `internal/decoder/nrppa` are left for separate work.

## Step 6 — Guard against reintroduction

Add a test that walks every known `ProtocolIEID` and `ProcedureCode` and asserts the decoder's rendered label equals the codec's `ProtocolIEIDName` / `ProcedureCodeName`, and the same for the NAS enums. A reintroduced local table fails it.

## Verification

- `go test ./...`. Regenerate the 35 golden fixtures in `internal/decoder/nas/testdata/golden/` with `go test ./internal/decoder/nas/ -run TestDecoderGolden -update` and review the diff against the names corrected in Steps 2 and 5.
- `go build`, `go vet` and `golangci-lint run` at the repo root and for each of the `ngap`, `s1ap`, `nas` modules.
- The `3gpp-server` conformance suite: `String()` output is public API of three published modules and feeds error text.
- `rg 'Names\s*=\s*map\[' internal/decoder/` returns empty.

## Scope

Steps 0-6 touch vocabulary lookup only. The decoder's display structs, per-IE builders, and value formatting keep their current shape.
