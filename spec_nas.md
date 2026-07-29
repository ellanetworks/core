# spec_nas: remaining work

## free5gc/nas removal (shipped binary already free5gc/nas-free)

1. `internal/tester` → `fgs`/`eps`. **In progress:**
   - DONE — UE crypto core: `free5gc/nas/security` removed (→ `nas/common` + `fgs.Protect`/`Unprotect`; `common.Count`; `ue/security.go` alg map). Round-trip unit test.
   - TODO — UE message currency: `*nas.Message` + `nasType.*` + `nasMessage.*` (builders `build_*.go`, handlers `handle_*.go`, dispatch `nas_decode.go`/`ue.go` struct fields, `msg_name.go`, `cause.go`) → fgs typed structs / `fgs.ParseX`.
   - TODO — scenarios (`scenarios/gnb`, `scenarios/enb`, `scenarios/common`), validators (`testutil/validate/*`, `testutil/nas.go`, `testutil/procedure/*`, `testutil/security.go`, `testutil/timer.go`), radio (`gnb`/`enb`/`air` message-type constants).
2. Test files still building inputs with `free5gc/nas` → `fgs`/`eps`: `internal/amf/*_test.go`, `internal/amf/nas/*_test.go`, `internal/amf/ngap/*_test.go`, `internal/decoder/nas/*_test.go` (incl. `golden_test.go` corpus builder), `internal/smf/lifecycle_test.go`.
3. DONE — `pkg/runtime`: dropped the `nasLogger` import. Shipped binary is now free of `github.com/free5gc/nas`.
4. Drop `github.com/free5gc/nas`: `go mod tidy`; confirm gone from `go.mod`/`go.sum`. (Blocked on 1–2.)

## 4G/5G decoder symmetry — DONE

5. DONE — removed redundant `extended_protocol_discriminator` / `spare_half_octet_and_security_header_type` echoes from all 19 5G per-message structs; GSM structs moved `pdu_session_id`/`pti` into `gsm_header` (mirroring 4G `esm_header`). Shape now symmetric with 4G.
6. DONE — 5G plain unrecognized message type renders the header with the `Unknown` enum and no error, matching 4G.
   (+ EnumField made non-generic `Value int64` across all decoders — nas/eps/ngap/s1ap/nrppa/lpp now uniform; arbitrary per-decoder width was not 3GPP-mandated.)

## Constraints — mandated deviations (spec-verified; do NOT "fix" for symmetry)

- `fgs.ExtendedProtocolDiscriminator` full octet vs `eps.ProtocolDiscriminator` half octet — TS 24.007 §11.2.3.1.1/.1A.
- `EncodeGPRSTimer3` 5G-only (§10.5.7.4a); `ServiceRequestShortMAC` / `SHTServiceRequest` 4G-only (§8.2.25).
- Per-RAT IEs: Session-AMBR / QoS-rules / S-NSSAI / DNN / PDU-address / access-type (5G) vs APN-AMBR / TFT (4G).
- `gmm`/`gsm` ↔ `emm`/`esm` tokens.
- `fgs.ParsePCOContainerIDs` vs `eps.ParseProtocolConfigurationOptions`.
- 5G-GUTI / 5G-S-TMSI via `etsi.GUTI5G` (4G has none).
- `DecodePlainGmm` STATUS #96/#97/#111 classification vs MME peek-only silence — §7 permits both ("should", not "shall").

## Validation

- `go build` / `golangci-lint` / `go test` clean, all modules.
- `nas/fgs` + `nas/eps`: round-trip / golden / fuzz + cipher/integrity vectors.
- `INTEGRATION=1 go test ./integration/...` green.
- No config/API/schema change; decoder-JSON changes only where a symmetry item (5/6) requires it.
