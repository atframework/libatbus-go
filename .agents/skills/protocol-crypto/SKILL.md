---
name: protocol-crypto
description: "ECDH key exchange, encryption/compression algorithms, message framing, and access token authentication in libatbus-go. Use when: working with ConnectionContext, handshake flow, cipher negotiation, pack/unpack, cross-language crypto vectors, access_data signatures."
---

# Protocol & Crypto — libatbus-go

## Protocol Wire Format

Protobuf v3 definition: `protocol/libatbus_protocol.proto`

### Message Structure

- **message_head** — version, type, sequence, source_bus_id, crypto metadata, compression metadata, body_size
- **message_body** — oneof: `custom_command_req/rsp`, `data_transform_req/rsp`, `node_register_req/rsp`, `node_ping_req`, `node_pong_rsp`, `handshake_confirm`
- **register_data** — bus_id, pid, hostname, channels, supported schemas/compression, access_key, crypto_handshake
- **forward_data** — from, to, router path, content, flags (REQUIRE_RSP)
- **ping_data** — time_point, crypto_handshake (carries ECDH public key)
- **crypto_handshake_data** — sequence, key exchange type, KDF types, cipher algorithms, public_key, iv_size, tag_size

Protocol version: `ATBUS_PROTOCOL_VERSION = 3`, minimum: `ATBUS_PROTOCOL_MINIMAL_VERSION = 3`.

### Frame Encoding

Wire frame: `[varint(header_len)][protobuf_header][body][padding]`

**Pack order**: serialize body → compress (if size ≥ threshold) → encrypt (random IV) → serialize header → prepend varint length.

**Unpack order**: read varint → parse header → decrypt → decompress → parse body.

Control messages (register, ping/pong, handshake_confirm) are **never** encrypted or compressed.

## ECDH Key Exchange Flow

Implementation: `impl/atbus_connection_context.go`

1. **Client** generates ECDH keypair, sends public key + supported algorithms in `ping_data.crypto_handshake`
2. **Server** generates its keypair, computes shared secret, selects best mutual algorithm, responds in `pong`
3. **Both** derive symmetric key + IV via HKDF-SHA256 from the shared secret
4. **Client** sends `handshake_confirm` to signal cipher switch
5. **Server** switches `receive_cipher` upon confirm receipt

### Key Refresh

Default interval: 3 hours (`CryptoKeyRefreshInterval`). Periodic re-handshake via ping/pong cycle.

## Supported Algorithms

### Key Exchange

| Algorithm         | Go Support    |
| ----------------- | ------------- |
| X25519            | `crypto/ecdh` |
| SECP256R1 (P-256) | `crypto/ecdh` |
| SECP384R1 (P-384) | `crypto/ecdh` |
| SECP521R1 (P-521) | `crypto/ecdh` |

### Symmetric Ciphers

| Algorithm               | Mode           | Go Package                             |
| ----------------------- | -------------- | -------------------------------------- |
| XXTEA                   | Block          | `golang.org/x/crypto` (custom impl)    |
| AES-128/192/256-CBC     | PKCS#7 padding | `crypto/aes` + `crypto/cipher`         |
| AES-128/192/256-GCM     | AEAD           | `crypto/aes` + `crypto/cipher`         |
| ChaCha20                | Stream (pure)  | `golang.org/x/crypto/chacha20`         |
| ChaCha20-Poly1305-IETF  | AEAD           | `golang.org/x/crypto/chacha20poly1305` |
| XChaCha20-Poly1305-IETF | AEAD           | `golang.org/x/crypto/chacha20poly1305` |

### Compression

| Algorithm | Go Package                           |
| --------- | ------------------------------------ |
| Zstd      | `github.com/klauspost/compress/zstd` |
| LZ4       | `github.com/pierrec/lz4/v4`          |
| Snappy    | `github.com/golang/snappy`           |

Compression levels: STORAGE, FAST, LOW_CPU, BALANCED, HIGH_RATIO, MAX_RATIO.

## Access Token Authentication

Implementation: `message_handle/atbus_message_handler.go`

### Signature Format

**Without crypto**:

```
"{timestamp}:{nonce1}-{nonce2}:{bus_id}"
```

**With crypto**:

```
"{timestamp}:{nonce1}-{nonce2}:{bus_id}:{key_exchange_type}:{hex(sha256(pubkey))}"
```

- Algorithm: HMAC-SHA256
- Multiple access tokens supported (up to `AccessTokenMaxNumber = 5`) for zero-downtime key rotation
- Timestamp tolerance: ±300 seconds

### Test Vectors

Binary test vectors in `message_handle/testdata/`:

- `signature_simple_token.bytes` / `.json` — Simple token signature verification
- `signature_binary_token.bytes` / `.json` — Binary token signature verification

## Cross-Language Parity

Crypto and compression test vectors in `impl/testdata/` ensure Go output matches C++ `libatbus` byte-for-byte.
The `tools/testdata_sync_custom_cmd/` tool syncs vectors from the C++ implementation.

Key parity areas:

- ECDH shared secret derivation
- HKDF-SHA256 key expansion
- XXTEA block encryption (padding scheme)
- Pure ChaCha20 stream cipher (no AEAD)
- All AEAD ciphers (nonce + ciphertext + tag format)
- All compression algorithms (dictionary-less mode)
