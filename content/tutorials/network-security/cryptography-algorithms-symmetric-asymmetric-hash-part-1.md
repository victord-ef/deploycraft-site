---
title: "Cryptographic Algorithm Fundamentals — Symmetric, Asymmetric, Hash Functions, and Modes — Part 1"
date: 2026-09-01
description: "Understand the core cryptographic primitives used in modern security: AES and ChaCha20 for symmetric encryption, RSA and ECC for asymmetric operations, SHA-2/SHA-3 for integrity, and block cipher modes of operation."
cluster: "Network Security"
series: "Cryptography"
part: 1
difficulty: "intermediate"
duration: "50 min"
tags: ["cryptography", "aes", "rsa", "ecc", "sha", "network-security", "devsecops", "security-engineering"]
categories: ["tutorial"]
draft: false
toc: true
---

## What you will build

By the end of this tutorial you will understand how the cryptographic primitives used in every security protocol actually work: why AES-256-GCM is the preferred symmetric cipher, how RSA and elliptic curve cryptography differ and when to use each, what hash functions guarantee and what they do not, and how block cipher modes change the security properties of encryption. Part 2 applies these primitives in the context of TLS 1.3, X.509 certificates, and key management.

## Prerequisites

- Basic mathematics (modular arithmetic concepts at a high level)
- Familiarity with security concepts (confidentiality, integrity, authentication)

---

## The three types of cryptographic primitives

Modern cryptographic systems combine three families of primitives, each solving a different problem:

| Type | What it provides | Examples |
|---|---|---|
| **Symmetric** | Confidentiality — fast encryption with shared key | AES, ChaCha20 |
| **Asymmetric** | Key exchange, digital signatures — no shared secret needed | RSA, ECDSA, ECDH |
| **Hash functions** | Integrity — one-way fingerprint of data | SHA-256, SHA-3, BLAKE2 |

No real protocol uses just one type. TLS 1.3 uses asymmetric cryptography (ECDH) to establish a shared secret, symmetric cryptography (AES-GCM) to encrypt data, and hash functions (SHA-384) for message authentication and key derivation.

---

## Step 1 — Symmetric encryption: AES

AES (Advanced Encryption Standard) is a block cipher — it encrypts fixed 128-bit blocks of data using a shared key. Key sizes are 128, 192, or 256 bits. AES-256 is the current standard for sensitive data.

### How AES works (simplified)

AES operates on a 4×4 byte state matrix through 10–14 rounds (depending on key size), each applying four transformations:

1. **SubBytes** — each byte replaced by its lookup in a fixed S-box (non-linear substitution)
2. **ShiftRows** — rows cyclically shifted by 0, 1, 2, 3 positions
3. **MixColumns** — each column multiplied with a fixed matrix in GF(2⁸)
4. **AddRoundKey** — XOR with the round key derived from the original key

The combination of SubBytes (non-linearity) and MixColumns (diffusion) ensures that changing one bit of input affects all 128 bits of output after a few rounds — the **avalanche effect**.

```python
# Python demonstration using cryptography library
from cryptography.hazmat.primitives.ciphers import Cipher, algorithms, modes
from cryptography.hazmat.backends import default_backend
import os

key = os.urandom(32)     # 256-bit key
nonce = os.urandom(12)   # 96-bit nonce for GCM

cipher = Cipher(
    algorithms.AES(key),
    modes.GCM(nonce),
    backend=default_backend()
)
encryptor = cipher.encryptor()

plaintext = b"Sensitive data"
ciphertext = encryptor.update(plaintext) + encryptor.finalize()
tag = encryptor.tag    # 128-bit authentication tag

print(f"Ciphertext: {ciphertext.hex()}")
print(f"Auth tag:   {tag.hex()}")
```

### AES modes of operation

A mode of operation defines how AES (which encrypts one 128-bit block at a time) handles messages of arbitrary length:

**ECB (Electronic Codebook) — never use:**
Each block encrypted independently. Identical plaintext blocks produce identical ciphertext blocks. Pattern leakage makes ECB catastrophically insecure for any structured data.

**CBC (Cipher Block Chaining):**
Each block XORed with the previous ciphertext block before encryption. Requires padding to fill the last block. Vulnerable to padding oracle attacks if padding errors are observable. Requires a separate MAC (HMAC) for integrity — encryption alone does not prevent tampering.

**CTR (Counter Mode):**
Generates a keystream by encrypting successive counter values and XORing with plaintext. No padding required — turns AES into a stream cipher. Secure but requires a separate MAC for integrity.

**GCM (Galois/Counter Mode) — recommended:**
CTR mode + GHASH (polynomial authentication in GF(2¹²⁸)). Provides **authenticated encryption** — the decryption fails with an explicit error if the ciphertext has been tampered with. The authentication tag covers both the ciphertext and associated data (AAD — metadata like packet headers that should be authenticated but not encrypted).

AES-256-GCM is the only AES mode recommended for new systems in 2026.

### ChaCha20-Poly1305

ChaCha20 is a stream cipher using only XOR, addition, and rotation — no lookup tables. Poly1305 provides the authentication tag.

**Why ChaCha20-Poly1305 matters:**
- AES requires hardware acceleration (AES-NI instructions) for performance. Without hardware support, AES is 3–5× slower than ChaCha20.
- Mobile devices and embedded systems often lack AES-NI.
- ChaCha20-Poly1305 performs comparably to software AES on all hardware.
- TLS 1.3 mandates support for both AES-256-GCM and ChaCha20-Poly1305.

---

## Step 2 — Asymmetric cryptography: RSA

RSA is based on the difficulty of factoring the product of two large primes.

### Key generation

```
1. Choose two large primes p and q (each 2048+ bits)
2. Compute n = p × q  (the modulus — this is public)
3. Compute φ(n) = (p-1)(q-1)
4. Choose e = 65537 (public exponent — standard choice)
5. Compute d such that e×d ≡ 1 (mod φ(n))  (private exponent)

Public key:  (n, e)
Private key: (n, d)   — p, q, φ(n) discarded after key generation
```

### Encryption and decryption

```
Encrypt:  C = M^e mod n    (M = message, C = ciphertext)
Decrypt:  M = C^d mod n
```

Factoring n back into p and q is computationally infeasible for key sizes ≥ 2048 bits (currently). An attacker who knows p and q can compute d and decrypt everything.

**RSA in practice — NEVER encrypt data directly with RSA:**
RSA is slow and has size limits (max message size = key size - padding). Real protocols use RSA to encrypt or sign a symmetric key, which then encrypts the actual data. This is called **hybrid encryption**.

### RSA signature

```
Sign:   S = H(M)^d mod n    (sign the hash, not the raw message)
Verify: H(M) == S^e mod n
```

RSA signing uses the private key to sign a hash of the message. Verification uses the public key. Anyone with the public key can verify the signature; only the private key holder can create it.

**RSA key size recommendations:**

| Key size | Security level | Status |
|---|---|---|
| 1024-bit | ~80 bits | Broken — do not use |
| 2048-bit | ~112 bits | Minimum acceptable |
| 3072-bit | ~128 bits | Recommended |
| 4096-bit | ~140 bits | High security / long-term keys |

---

## Step 3 — Asymmetric cryptography: Elliptic Curve (ECC)

ECC achieves the same security as RSA with dramatically smaller key sizes because breaking ECC requires solving the elliptic curve discrete logarithm problem, which is harder than integer factorisation.

**Equivalent security levels:**

| RSA key size | ECC key size | Security bits |
|---|---|---|
| 1024-bit | 160-bit | 80 (broken) |
| 2048-bit | 224-bit | 112 |
| 3072-bit | 256-bit | 128 |
| 7680-bit | 384-bit | 192 |
| 15360-bit | 521-bit | 260 |

A 256-bit ECC key provides the same security as a 3072-bit RSA key, with much faster operations and smaller signatures.

### Common curves

| Curve | Also known as | Use case |
|---|---|---|
| P-256 | secp256r1, prime256v1 | TLS, JWT signatures, general use |
| P-384 | secp384r1 | High-security government / FIPS |
| P-521 | secp521r1 | Very high security |
| Curve25519 | X25519 (DH), Ed25519 (signatures) | WireGuard, SSH, modern TLS |
| secp256k1 | — | Bitcoin (not for general TLS) |

Curve25519 is the preferred curve for new systems: no known weaknesses, faster than NIST curves, immune to timing side-channels by construction.

### ECDH — Elliptic Curve Diffie-Hellman

ECDH enables two parties to establish a shared secret without transmitting it:

```
Alice generates keypair:  a (private), A = a×G (public, G=generator point)
Bob generates keypair:    b (private), B = b×G (public)

Alice sends A to Bob, Bob sends B to Alice.

Alice computes: S = a×B = a×(b×G) = (a×b)×G
Bob computes:   S = b×A = b×(a×G) = (a×b)×G

Both arrive at the same S without ever transmitting a×b.
An observer sees only A and B — recovering a or b requires solving ECDLP.
```

ECDH generates a shared secret. That secret is fed into a KDF (Key Derivation Function) to produce symmetric encryption keys.

### ECDSA — Elliptic Curve Digital Signature Algorithm

Used for signing: SSH keys, X.509 certificates (cert-manager ECDSA), JWT signatures.

```bash
# Generate an ECDSA P-256 key pair
openssl ecparam -name prime256v1 -genkey -noout -out private.key
openssl ec -in private.key -pubout -out public.key

# Sign a file
openssl dgst -sha256 -sign private.key -out signature.der data.txt

# Verify the signature
openssl dgst -sha256 -verify public.key -signature signature.der data.txt
# Verified OK
```

---

## Step 4 — Hash functions

A cryptographic hash function maps arbitrary-length input to a fixed-length output (the digest) with three properties:

- **Pre-image resistance:** Given H(x), it is infeasible to find x
- **Second pre-image resistance:** Given x and H(x), it is infeasible to find y ≠ x such that H(y) = H(x)
- **Collision resistance:** It is infeasible to find any pair (x, y) where x ≠ y and H(x) = H(y)

Hash functions do **not** encrypt — they are one-way. They are used for:
- Data integrity (does the file match the expected hash?)
- Digital signatures (sign the hash, not the raw message)
- Password storage (store hash, not plaintext)
- Key derivation (generate keys from a master secret)
- Message authentication codes (HMAC)

### SHA-2 family

| Function | Output size | Status |
|---|---|---|
| SHA-256 | 256 bits | Standard — use for most purposes |
| SHA-384 | 384 bits | Higher security, preferred for TLS HMAC |
| SHA-512 | 512 bits | High security, larger output |
| SHA-512/256 | 256 bits | SHA-512 truncated — faster on 64-bit hardware than SHA-256 |

SHA-256 is the workhorse. SHA-384 is preferred for TLS 1.3 cipher suites (`TLS_AES_256_GCM_SHA384`).

### SHA-3 family (Keccak)

SHA-3 uses a completely different design (sponge construction vs Merkle-Damgård). It is not faster than SHA-2 but provides a cryptographic diversity guarantee — if a structural weakness is found in SHA-2, SHA-3 provides an alternative.

### BLAKE2 and BLAKE3

BLAKE2 is faster than SHA-256 on software implementations while maintaining security. BLAKE3 is parallelisable across CPU cores. Both are used in file integrity tools and modern password hashing schemes.

### What SHA-256 is not

SHA-256 alone is **not** a MAC (Message Authentication Code). A MAC requires a key. An attacker who can append to a message can also append to its SHA-256 hash (length extension attack). Use HMAC-SHA-256 (keyed hash) for message authentication.

---

## Step 5 — HMAC and key derivation

### HMAC (Hash-based MAC)

HMAC uses a secret key with a hash function to produce an authenticated digest:

```
HMAC-SHA256(key, message) = SHA256((key XOR opad) || SHA256((key XOR ipad) || message))
```

Where `opad` and `ipad` are fixed padding constants. HMAC prevents length extension attacks and authenticates both the message and the key holder.

```python
import hmac, hashlib

key = b"secret-key"
message = b"data to authenticate"

mac = hmac.new(key, message, hashlib.sha256).hexdigest()
print(f"HMAC-SHA256: {mac}")

# Verify
expected = hmac.new(key, message, hashlib.sha256).digest()
received = bytes.fromhex(mac)
if hmac.compare_digest(expected, received):    # constant-time comparison
    print("Valid")
```

Always use `hmac.compare_digest()` — not `==` — to prevent timing attacks where an attacker learns how many bytes matched before the comparison failed.

### HKDF — HMAC-based Key Derivation Function

HKDF derives multiple keys from a single source of randomness (like an ECDH shared secret):

```
HKDF(IKM, salt, info, length):
  PRK = HMAC-SHA256(salt, IKM)             # extract phase
  OKM = HMAC-SHA256(PRK, info || 0x01)    # expand phase
  → returns `length` bytes of keying material
```

TLS 1.3 uses HKDF to derive all session keys from the ECDH shared secret.

---

## Step 6 — Putting it together: why algorithms are combined

No single primitive solves all problems. A practical encrypted channel needs:

```
1. Key agreement:    ECDH (asymmetric) → shared secret
2. Key derivation:   HKDF-SHA384 → symmetric keys
3. Encryption:       AES-256-GCM (symmetric) → confidentiality + integrity
4. Authentication:   ECDSA-P256 (asymmetric) → verify peer identity
5. Integrity:        GCM authentication tag (AEAD) → detect tampering
```

This is exactly what TLS 1.3 does on every connection — and IPSec's IKEv2 follows the same pattern.

---

## Step 7 — Deprecated algorithms and why

| Algorithm | Broken because | Replace with |
|---|---|---|
| MD5 | Collision attacks demonstrated since 2004 | SHA-256 |
| SHA-1 | Practical collision attack (SHAttered, 2017) | SHA-256 |
| DES | 56-bit key — exhaustive search in hours | AES-256 |
| 3DES | Sweet32 birthday attack at 64-bit block boundary | AES-256-GCM |
| RC4 | Statistical biases leak plaintext | ChaCha20-Poly1305 |
| RSA-PKCS1v1.5 | Bleichenbacher padding oracle | RSA-OAEP or ECDH |
| Diffie-Hellman 1024-bit | Logjam — precomputed discrete logs | ECDH P-256 or Curve25519 |

---

## What you have built

- AES block cipher internals — SubBytes, ShiftRows, MixColumns, AddRoundKey — and why AES-256-GCM is the standard choice
- Block cipher modes — ECB (broken), CBC (padding oracles), CTR (no integrity), GCM (AEAD, recommended)
- ChaCha20-Poly1305 — why it matters for hardware without AES-NI
- RSA — key generation, encryption, signature, and key size recommendations
- ECC — ECDH for key exchange, ECDSA for signatures, Curve25519 vs NIST curves
- Hash functions — SHA-2 family, SHA-3, BLAKE2, and the properties each guarantees
- HMAC for message authentication and constant-time comparison
- HKDF for key derivation from shared secrets
- A clear map of which deprecated algorithms to replace and with what

In [Part 2](/tutorials/network-security/applied-cryptography-tls13-certificate-chains-part-2/) you will trace how these primitives combine in TLS 1.3 — the full handshake, certificate chain validation, session resumption, and practical key management for server operators.
