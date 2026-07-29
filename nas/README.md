# nas

A 3GPP NAS (Non-Access Stratum) codec in Go, covering both 5GS (TS 24.501) and
EPS (TS 24.301): message and information-element encoding and decoding, NAS
security framing, and the integrity and ciphering algorithms.

Depends only on the standard library.

## Install

```
go get github.com/ellanetworks/core/nas
```

## Packages

| Package       | Contents                                                             |
| ------------- | -------------------------------------------------------------------- |
| `nas`         | Wire primitives, elements both generations share, security algorithms |
| `nas/fgs`     | 5GS: 5GMM and 5GSM messages and elements (TS 24.501)                  |
| `nas/eps`     | EPS: EMM and ESM messages and elements (TS 24.301)                    |
| `nas/nastest` | Builders for malformed PDUs, for negative tests                       |

`fgs` and `eps` are deliberately symmetric: the same concept carries the same
type and field names in both, except where 3GPP defines a different structure.

## Decoding

`ParseMessage` decodes any plain message of its generation; dispatch on the
concrete type.

```go
msg, err := fgs.ParseMessage(pdu)
if err != nil && !nas.SoftOnly(err) {
    return err
}

switch m := msg.(type) {
case *fgs.RegistrationRequest:
    handleRegistrationRequest(m)
case *fgs.ULNASTransport:
    handleULNASTransport(m)
case *fgs.UnknownMessage:
    sendStatus(m.Type) // a type this library does not model
}
```

`eps.ParseMessage` takes the direction as well, because TS 24.301 gives DETACH
REQUEST one message type for both directions where TS 24.501 assigns two.

A parser returns a usable message alongside a non-nil error in two cases: only
optional elements failed to decode — TS 24.501 §7.7.1 and TS 24.301 §7.7.1
require a receiver to treat those as not present — or the message type is one
the library does not model, which arrives as an `UnknownMessage` so the receiver
can still answer with a STATUS. `nas.SoftOnly` reports both, and a hard failure
returns a nil message: **a message comes back exactly when the error is soft**.

Each message also has its own `Parse<Message>` when the caller already knows
what to expect.

## Encoding

```go
accept := &fgs.RegistrationAccept{
    RegistrationResult: fgs.RegistrationResult3GPP,
    GUTI:               &guti,
}

pdu, err := accept.MarshalBinary()
```

Construct messages with keyed literals, and do not compare them with `==`.

## Round trips

Decoding never discards an element. An element the library does not model is
preserved among the message's `Unrecognized` elements and re-encoded with the
rest, so:

- **Lossless** — everything decoded re-encodes.
- **Stable** — re-encoding an encoding yields the same octets.
- **Byte-identical** for input already in the order its spec table defines.

Input whose optional elements arrive out of that order is canonicalized on the
first encode. `MarshalBinary(Parse(b))` therefore equals `b` for canonical input, not
for all input.

## Buffers

A parsed value owns its memory. The input buffer may be reused or mutated as
soon as `Parse` returns.

## Security

Build a security context once, then protect and verify with it:

```go
sc, err := nas.NewSecurityContext(nas.SecurityContextOptions{
    Integrity:    nas.IntegrityAES,
    Ciphering:    nas.CipheringAES,
    IntegrityKey: kNASint,
    CipherKey:    kNASenc,
})

wire, err := fgs.Protect(plain, fgs.SHTIntegrityProtectedCiphered, count, nas.DirectionDownlink, sc)

plain, sht, err := fgs.Unprotect(pdu, count, nas.DirectionUplink, sc)
```

The constructor refuses a context that could not protect anything: an algorithm
the library does not implement, an all-zero key, or null integrity without an
explicit `AllowNullIntegrity`. A context is the only way to reach an algorithm —
`Integrity` and `Cipher` are sealed — so NAS security cannot silently degrade.

The NAS COUNTs belong to the connection, not the keys: take the downlink count
from a `nas.DownlinkCounter`, which refuses to reuse one, and estimate the uplink
count with a `nas.UplinkCounter`, which is the replay check. Commit an uplink
count only after `Unprotect` returns.

The library owns wire framing and the cryptographic transforms. The caller owns
the key store, the security context, and the decision to install one.

## 3GPP releases

Written against Release 18: TS 24.501 v18.6.0, TS 24.301 v18.9.0,
TS 24.007 v18.2.0, TS 24.008 v18.8.0, TS 33.501 v18.6.0, TS 33.401 v18.3.0, and
TS 23.003 v16.3.0.

## Compatibility

At v0; the API still moves. From v1.0.0, modelling a new element adds a field, a
field's type never changes, and type and field names are stable. `nas/nastest`
is exempt: it exists to build malformed PDUs, so its surface grows with the
negative tests.

## Licence

Business Source License 1.1 — see [LICENSE](LICENSE).
