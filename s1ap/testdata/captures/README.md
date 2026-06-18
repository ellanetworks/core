# Golden-vector captures

Real S1AP PDUs used as the correctness oracle for the hand-written codec.

Capture from a reference stack, export the S1AP
payloads, and store them here as raw bytes (hex or binary) alongside the expected
decoded form. Tests assert:

- decode-matches-struct (the bytes decode to the expected typed PDU), and
- re-encode-equals-bytes (re-marshalling the struct reproduces the original bytes).

These also seed the fuzz corpus for the decoder (invariant: never panics).

Naming: `<procedure>_<direction>.bin` (e.g. `s1setup_request.bin`,
`s1setup_response.bin`).
