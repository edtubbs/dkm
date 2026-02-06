# OP-TEE Enclave Integration - Minimal Changes

## Summary

DKM now integrates with libdogecoin's OP-TEE secure enclave for seedphrase storage using the `optee_libdogecoin` CLI tool.

## Core Changes

### 1. Enclave Package (`internal/enclave/optee.go`)

New Go wrapper around `optee_libdogecoin` CLI:

```go
// Core operations
GenerateMnemonic(password string) ([]string, error)
GenerateExtendedPublicKey(account, changeLevel int, password, customPath string) (string, error)
GenerateAddress(account, changeLevel, addressIndex int, password, customPath string) (string, error)

// Delegate operations
DelegateKey(account int, delegatePassword, password, customPath string) error
ExportDelegateKey(account int, delegatePassword string) (string, error)

// Utility
HasMnemonic(password string) bool
```

### 2. Key Manager Integration (`internal/keymgr/keymgr.go`)

`CreateKey()` now tries OP-TEE enclave first, falls back to local generation if unavailable.

### 3. Key Derivation Alignment

- Master key: path `m` (for verification)
- Delegates: path `m/1000'/2'/N'` (matching DKM's namespace)
- Uses `-h <path>` flag for custom BIP32 paths
- Uses `-p <password>` flag for main password
- Uses `-d <delegate_password>` flag for delegate operations

## CLI Commands Generated

```bash
# Generate mnemonic with delegate flag
optee_libdogecoin -c generate_mnemonic -p <password> -f "delegate"

# Verify master key exists
optee_libdogecoin -c generate_extended_public_key -h m -p <password>

# Delegate key at DKM's path
optee_libdogecoin -c delegate_key -o 0 -d <delegate_pass> -h "m/1000'/2'/0'" -p <password>

# Export delegate
optee_libdogecoin -c export_delegate_key -o 0 -d <delegate_pass>
```

## Documentation

- **README.md**: User-facing overview (96 lines)
- **docs/OPTEE-INTEGRATION.md**: Technical details (280 lines)

## Build

```bash
make              # Standard build
nix build         # Nix standard
nix build .#dkm-optee  # Nix with optee_libdogecoin in PATH
```

All features work with or without OP-TEE available (graceful fallback).
