# Doge Key Manager

The DKM holds your encrypted master key and generates (derives) private-public
keypairs for pups and other parts of the DogeBox ecosystem.

## Key Store

Keys are encrypted at rest with the DogeBox password and stored on disk.

Passwords are first hashed using Argon2 memory-hard KDF (Argon2id variant)
with parameters time=3, memory=64M, threads=4 and the BLAKE2b hash function
as recommended in RFC 9106.

The password-derived hash is then used to encrypt the master key with
ChaCha20 cypher and Poly1305 Authenticated Encryption (AE) scheme.

Keys in DKM are only in memory while they are actively being used for
Authentication or key derivation.

## OP-TEE Secure Enclave Integration

DKM integrates with the **libdogecoin OP-TEE Trusted Application** to store 
the seedphrase (mnemonic) in a secure enclave. When the `optee_libdogecoin` 
CLI tool is available, DKM will:

- Generate the mnemonic within the OP-TEE secure enclave
- Store it in encrypted secure storage within the enclave
- Never expose the mnemonic outside the secure world (except during initial generation for backup)

This provides an additional layer of security by isolating the most sensitive
cryptographic material (the mnemonic) from the host system, even if the host
is compromised.

### How It Works

DKM calls the `optee_libdogecoin` command-line tool to interact with the OP-TEE 
Trusted Application. The tool handles all communication with the secure enclave:

```bash
# Generate mnemonic in enclave (called by DKM)
optee_libdogecoin -c generate_mnemonic -z

# Generate address from enclave-stored mnemonic
optee_libdogecoin -c generate_address -z -o 0 -l 0 -i 0
```

The `-z` flag skips YubiKey/TOTP authentication for simpler integration.

### Building with OP-TEE Support

To build DKM with OP-TEE support, use the `dkm-optee` package:

```bash
nix build .#dkm-optee
```

This will ensure `optee_libdogecoin` is available in the PATH when running DKM.

For development with OP-TEE:

```bash
nix develop .#optee
```

### Requirements

When using OP-TEE support, you need:

- `optee_libdogecoin` CLI tool (from libdogecoin-optee-host package)
- OP-TEE OS running on the system
- libdogecoin TA installed (UUID: 62d95dc0-7fc2-4cb3-a7f3-c13ae4e633c4)
- tee-supplicant service running

Without these components, DKM will automatically fall back to local mnemonic 
generation without user intervention.

### Fallback Behavior

DKM gracefully handles the absence of OP-TEE:

1. **Key Creation**: Attempts to use OP-TEE enclave first, falls back to local generation if unavailable
2. **Compatibility**: Works identically whether OP-TEE is present or not
3. **No Breaking Changes**: Existing DKM deployments continue to work unchanged

The master key derived from the mnemonic is still encrypted and stored locally
in both cases, ensuring compatibility across all deployments.

