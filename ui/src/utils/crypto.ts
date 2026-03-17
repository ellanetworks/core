function hexToBytes(hex: string): Uint8Array {
  const bytes = new Uint8Array(hex.length / 2);
  for (let i = 0; i < hex.length; i += 2) {
    bytes[i / 2] = parseInt(hex.substring(i, i + 2), 16);
  }
  return bytes;
}

function bytesToHex(bytes: Uint8Array): string {
  return Array.from(bytes)
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("");
}

/**
 * Compress a P-256 uncompressed point (65 bytes: 0x04 || x || y)
 * to SEC1 compressed format (33 bytes: 0x02/0x03 || x).
 */
function compressP256Point(uncompressed: Uint8Array): string {
  const x = uncompressed.slice(1, 33);
  const y = uncompressed.slice(33, 65);
  const prefix = y[31] % 2 === 0 ? 0x02 : 0x03;
  const compressed = new Uint8Array(33);
  compressed[0] = prefix;
  compressed.set(x, 1);
  return bytesToHex(compressed);
}

/**
 * Wrap a raw 32-byte P-256 private key in a minimal PKCS#8 DER envelope
 * so it can be imported via crypto.subtle.importKey("pkcs8", ...).
 */
function wrapP256PrivateKeyPKCS8(raw: Uint8Array): ArrayBuffer {
  // PKCS#8 header for P-256 (without public key)
  const header = new Uint8Array([
    0x30, 0x41, 0x02, 0x01, 0x00, 0x30, 0x13, 0x06, 0x07, 0x2a, 0x86, 0x48,
    0xce, 0x3d, 0x02, 0x01, 0x06, 0x08, 0x2a, 0x86, 0x48, 0xce, 0x3d, 0x03,
    0x01, 0x07, 0x04, 0x27, 0x30, 0x25, 0x02, 0x01, 0x01, 0x04, 0x20,
  ]);
  const result = new Uint8Array(header.length + raw.length);
  result.set(header);
  result.set(raw, header.length);
  return result.buffer;
}

/**
 * Derive Profile B (P-256) compressed public key from a 32-byte hex private key.
 * Uses the Web Crypto API. Returns SEC1 compressed format (66 hex chars).
 */
export async function derivePublicKeyB(privateKeyHex: string): Promise<string> {
  const privBytes = hexToBytes(privateKeyHex);
  const pkcs8 = wrapP256PrivateKeyPKCS8(privBytes);
  const key = await crypto.subtle.importKey(
    "pkcs8",
    pkcs8,
    { name: "ECDH", namedCurve: "P-256" },
    true,
    ["deriveBits"],
  );
  const exported = await crypto.subtle.exportKey("raw", key);
  return compressP256Point(new Uint8Array(exported));
}
