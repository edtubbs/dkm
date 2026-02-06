# OP-TEE Secure Enclave Integration

This document describes how DKM integrates with the libdogecoin OP-TEE Trusted Application.

## Overview

DKM integrates with OP-TEE by calling the `optee_libdogecoin` command-line tool, which is part of the `libdogecoin-optee-host` package. This tool provides a simple interface to the libdogecoin Trusted Application (TA) running in the secure world.

## Architecture

```
┌─────────────────────────────────────────────┐
│              DKM Application                │
│  ┌─────────────────────────────────────┐   │
│  │       Key Manager (keymgr)          │   │
│  │  - CreateKey()                      │   │
│  │  - Checks for optee_libdogecoin     │   │
│  └────────────┬────────────────────────┘   │
│               │                             │
│  ┌────────────▼────────────────────────┐   │
│  │    Enclave Package (enclave)        │   │
│  │  - OpteeTool wrapper                │   │
│  │  - Calls CLI via exec.Command       │   │
│  └────────────┬────────────────────────┘   │
└───────────────┼──────────────────────────────┘
                │ CLI invocation
┌───────────────▼──────────────────────────────┐
│       optee_libdogecoin CLI Tool            │
│       (libdogecoin-optee-host package)      │
└───────────────┬──────────────────────────────┘
                │ OP-TEE Client API (libteec)
┌───────────────▼──────────────────────────────┐
│           OP-TEE OS (Secure World)           │
│  ┌──────────────────────────────────────┐   │
│  │  libdogecoin TA                      │   │
│  │  UUID: 62d95dc0-7fc2-4cb3-...        │   │
│  │  - Generate mnemonic                 │   │
│  │  - Secure storage                    │   │
│  │  - BIP39/BIP32 operations            │   │
│  └──────────────────────────────────────┘   │
└──────────────────────────────────────────────┘
```

## CLI Interface

The `optee_libdogecoin` tool provides these commands:

### Generate Mnemonic
```bash
optee_libdogecoin -c generate_mnemonic -z
```
- Generates a new BIP39 mnemonic in the secure enclave
- Stores it in secure storage
- Returns the mnemonic (one-time only, for user backup)
- `-z` flag skips YubiKey/TOTP authentication

### Generate Address
```bash
optee_libdogecoin -c generate_address -z -o 0 -l 0 -i 0
```
- Generates a Dogecoin address from the stored mnemonic
- Uses BIP44 derivation path: m/44'/3'/account'/change/index
- `-o` = account, `-l` = change level, `-i` = address index
- Returns the address (e.g., "D...")

## Implementation Details

### Enclave Package (`internal/enclave/optee.go`)

The enclave package provides a Go interface to the CLI tool:

```go
type OpteeTool struct {
    binPath string
}

func NewOpteeTool(binPath string) (*OpteeTool, error)
func (t *OpteeTool) GenerateMnemonic() ([]string, error)
func (t *OpteeTool) GenerateAddress(account, changeLevel, addressIndex int) (string, error)
func (t *OpteeTool) HasMnemonic() bool
```

Key features:
- Uses `exec.Command` to invoke the CLI tool
- Parses stdout using regex to extract results
- Returns structured errors on failure
- Checks if tool is available using `exec.LookPath`

### Key Manager Integration

The `CreateKey()` method attempts to use the enclave:

1. Try to initialize `OpteeTool`
   - If fails → Use local mnemonic generation
2. Check if mnemonic already exists in enclave
   - If yes → Return ErrKeyExists
3. Generate mnemonic in enclave
   - If fails → Use local mnemonic generation
4. Derive master key from mnemonic
5. Encrypt and store master key locally

This ensures backward compatibility and graceful fallback.

## Build Configuration

The `flake.nix` provides two build variants:

### Standard Build
```bash
nix build .#dkm
```
- Normal build without OP-TEE
- Falls back to local generation if optee_libdogecoin not found

### OP-TEE Build
```bash
nix build .#dkm-optee
```
- Includes `libdogecoin-optee-host` package
- Wraps `dkm` binary to ensure `optee_libdogecoin` is in PATH
- Creates wrapper script:
  ```bash
  #!/bin/sh
  export PATH="/nix/store/.../bin:$PATH"
  exec $out/bin/.dkm-wrapped "$@"
  ```

### Development Shell
```bash
nix develop .#optee
```
- Provides Go 1.25
- Includes `optee_libdogecoin` in PATH
- Sets up environment for development

## Runtime Requirements

For OP-TEE to work, the system needs:

1. **OP-TEE OS**: Trusted execution environment running
2. **tee-supplicant**: Service to facilitate communication with TEE
3. **libdogecoin TA**: Installed at `/lib/optee_armtz/62d95dc0-7fc2-4cb3-a7f3-c13ae4e633c4.ta`
4. **optee_libdogecoin**: CLI tool in PATH

Example `tee-supplicant` configuration (NixOS):
```nix
services.tee-supplicant = {
  enable = true;
  trustedApplications = [
    "${libdogecoin."libdogecoin-optee-ta"}/ta/62d95dc0-7fc2-4cb3-a7f3-c13ae4e633c4.ta"
  ];
};
```

## Security Model

### What's Protected
- **Mnemonic (seedphrase)**: Stored in OP-TEE secure storage
- **Enclave operations**: Run in ARM TrustZone secure world
- **Storage encryption**: Handled by OP-TEE OS

### What's Not Protected
- **Master key**: Still encrypted and stored in SQLite (same as before)
- **Derived keys**: Generated on-demand in normal world
- **Session tokens**: In-memory in normal world

### Why This Design?

1. **Compatibility**: Master key storage unchanged for backward compatibility
2. **Performance**: Derived keys don't require enclave roundtrips
3. **Simplicity**: Only the most sensitive data (mnemonic) in enclave
4. **Pragmatic**: Balances security with usability

## Output Parsing

The CLI tool outputs structured text that we parse:

### Mnemonic Generation Output
```
Mnemonic generated: word1 word2 word3 ... word24
```
Regex: `Mnemonic generated:\s*(.+)`

### Address Generation Output
```
Address generated: D5oXv9QKNnSF6rCvXKQVhtJ7aDLSzJTMwJ
```
Regex: `Address generated:\s*(\S+)`

## Error Handling

The integration uses a fallback strategy:

```
Try OP-TEE
    ↓
Available? → No → Use local generation
    ↓ Yes
Generate in enclave
    ↓
Success? → No → Use local generation
    ↓ Yes
Return mnemonic
```

This ensures DKM works whether OP-TEE is present or not.

## Comparison with CGO Approach

**Previous approach** (incorrect):
- Direct CGO bindings to libteec
- Implemented TEE client code in Go
- Required C headers and libraries at build time
- Complex build dependencies

**Current approach** (correct):
- Call external CLI tool via exec.Command
- Simple subprocess invocation
- No CGO, no C headers needed
- Matches actual libdogecoin usage pattern

## References

- Example usage: https://github.com/edtubbs/pups/blob/spv-enclave/spv_enclave/pup.nix
- libdogecoin enclave docs: https://github.com/dogecoinfoundation/libdogecoin/blob/0.1.5-dev/doc/enclaves.md
- libdogecoin package: https://github.com/Dogebox-WG/dogebox-nur-packages/blob/main/pkgs/libdogecoin/default.nix
