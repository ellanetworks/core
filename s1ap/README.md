# s1ap

A pure Go codec for the S1 Application Protocol (S1AP), the control-plane protocol between an eNodeB and the MME over the S1-MME interface in EPC/LTE.

It is a `[]byte ⇄ typed PDU` library: no transport, no SCTP, no state, no procedure logic. `Marshal` and `Unmarshal` are pure and concurrency-safe.

## Specifications

- **3GPP TS 36.413** v18.1.0 (ETSI TS 136 413) — S1AP message and IE definitions.
- **ITU-T X.691** (02/2021) — Aligned Packed Encoding Rules (APER).

Both specs are © ETSI/3GPP/ITU-T; download them at the pinned versions from their official portals. The `aper` package implements only the X.691 subset S1AP uses.

## Install

```sh
go get github.com/ellanetworks/s1ap
```

## Usage

Each message type provides `Marshal` (typed struct → S1AP-PDU bytes) and a `Parse*` function (open-type payload → typed struct). `Unmarshal` decodes the outer PDU envelope into an `InitiatingMessage`,  `SuccessfulOutcome`, or `UnsuccessfulOutcome`.

```go
b, err := (&s1ap.Paging{ /* ... */ }).Marshal()
if err != nil {
    // ...
}

pdu, err := s1ap.Unmarshal(b)
if err != nil {
    // ...
}

if im, ok := pdu.(*s1ap.InitiatingMessage); ok && im.ProcedureCode == s1ap.ProcPaging {
    msg, err := s1ap.ParsePaging(im.Value)
    // ...
}
```
