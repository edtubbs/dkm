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
optee_libdogecoin -c generate_mnemonic -p <password> -f "delegate"
```
- Generates a new BIP39 mnemonic in the secure enclave
- Stores it in secure storage
- Returns the mnemonic (one-time only, for user backup)
- `-p` flag provides the password for the mnemonic seedphrase
- `-f "delegate"` flag enables delegation features for the mnemonic
- `-z` flag can be added to enable YubiKey authentication (not used by DKM)

### Generate Extended Public Key
```bash
optee_libdogecoin -c generate_extended_public_key -h <custom_path> -p <password>
```
- Generates an extended public key from the stored mnemonic
- `-h` flag specifies a custom BIP32 derivation path (e.g., `m`, `m/1000'/2'`)
- DKM uses path `m` to verify the master key
- DKM uses path `m/1000'/2'` for delegate/pup namespace
- Returns the extended public key

### Generate Address
```bash
optee_libdogecoin -c generate_address -o 0 -l 0 -i 0 -p <password>
# OR with custom path
optee_libdogecoin -c generate_address -h <custom_path> -p <password>
```
- Generates a Dogecoin address from the stored mnemonic
- Uses BIP44 derivation path: m/44'/3'/account'/change/index
- `-o` = account, `-l` = change level, `-i` = address index
- `-h` = custom path (alternative to -o/-l/-i)
- `-p` = password for authentication
- Returns the address (e.g., "D...")

### Delegate Key
```bash
optee_libdogecoin -c delegate_key -o <account> -d <delegate_password> -h <custom_path> -p <password>
```
- Delegates account keys in the enclave with delegate password protection
- `-o` = account number
- `-d` = delegate password for the delegated keys
- `-h` = custom path (e.g., `"m/1000'/2'/0'"` for DKM delegate namespace)
- `-p` = main password for authentication

### Export Delegate Key
```bash
optee_libdogecoin -c export_delegate_key -o <account> -d <delegate_password>
```
- Exports delegated account keys using the delegate password
- `-o` = account number
- `-d` = delegate password
- Returns the exported key data

## Implementation Details

### Enclave Package (`internal/enclave/optee.go`)

The enclave package provides a Go interface to the CLI tool:

```go
type OpteeTool struct {
    binPath string
}

func NewOpteeTool(binPath string) (*OpteeTool, error)
func (t *OpteeTool) GenerateMnemonic(password string) ([]string, error)
func (t *OpteeTool) GenerateExtendedPublicKey(account, changeLevel int, password string, customPath string) (string, error)
func (t *OpteeTool) GenerateAddress(account, changeLevel, addressIndex int, password string, customPath string) (string, error)
func (t *OpteeTool) DelegateKey(account int, delegatePassword, password, customPath string) error
func (t *OpteeTool) ExportDelegateKey(account int, delegatePassword string) (string, error)
func (t *OpteeTool) HasMnemonic(password string) bool
```

Key features:
- Uses `exec.Command` to invoke the CLI tool
- Parses stdout using regex to extract results
- Returns structured errors on failure
- Checks if tool is available using `exec.LookPath`
- Passes password via `-p` flag (not `-z` for YubiKey)
- **Supports custom key paths via `-h` flag** to align with DKM's BIP32 derivation

### DKM Key Derivation Alignment

DKM uses specific BIP32 paths:
- **Master key**: `m` (derived directly from mnemonic)
- **Pup/Delegate namespace**: `m/1000'/2'/N'` (where N is delegate index)

The enclave integration now uses the `-h` custom path flag to ensure alignment:
- `HasMnemonic()` uses path `m` to verify master key existence
- `DelegateKey()` can use path `m/1000'/2'/N'` for delegate operations
- Standard operations can still use BIP44 paths via `-o`, `-l`, `-i` flags

### Delegate Key Operations

DKM's delegate key operations can leverage the enclave:
- `CreateDelegate()` creates keys at `m/1000'/2'/N'` - can use `DelegateKey()` with custom path
- Delegate keys are protected by both the main password and a delegate-specific password
- Export operations use the delegate password for access control

### Key Manager Integration

The `CreateKey()` method attempts to use the enclave:

1. Try to initialize `OpteeTool`
   - If fails → Use local mnemonic generation
2. Check if mnemonic already exists in enclave (using password)
   - If yes → Return ErrKeyExists
3. Generate mnemonic in enclave with password
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

## NixOS Deployment Configuration

### Architecture Overview

DKM is a **system-level service** that needs tee-supplicant running at the system level to use OP-TEE. Pups are containerized applications that may also need their own tee-supplicant inside their containers.

**No conflict exists because:**
- System-level tee-supplicant serves DKM (host service)
- Container-level tee-supplicant serves pups (isolated containers)

### System-Level Configuration (For DKM)

Add to **`Dogebox-WG/os`** repository in file **`nix/dbx/dkm.nix`**:

```nix
{ config, pkgs, lib, ... }:
{
  # ... existing DKM service configuration ...
  
  # Enable tee-supplicant for DKM's OP-TEE integration
  services.tee-supplicant = {
    enable = true;
    trustedApplications = [
      # OP-TEE OS standard TAs for RK3588 platform
      "${pkgs.optee-os-rockchip-rk3588.devkit}/ta/023f8f1a-292a-432b-8fc4-de8471358067.ta"
      "${pkgs.optee-os-rockchip-rk3588.devkit}/ta/80a4c275-0a47-4905-8285-1486a9771a08.ta"
      "${pkgs.optee-os-rockchip-rk3588.devkit}/ta/f04a0fe7-1f5d-4b9b-abf7-619b85b4ce8c.ta"
      "${pkgs.optee-os-rockchip-rk3588.devkit}/ta/fd02c9da-306c-48c7-a49c-bbd827ae86ee.ta"
      
      # libdogecoin OP-TEE Trusted Application (for DKM operations)
      "${libdogecoin."libdogecoin-optee-ta"}/ta/62d95dc0-7fc2-4cb3-a7f3-c13ae4e633c4.ta"
    ];
  };
}
```

**Why:** DKM runs as a system service and directly calls `optee_libdogecoin`, which requires tee-supplicant at the system level.

### Pup-Level Configuration (For Containerized Apps)

Individual pups also configure tee-supplicant in their `pup.nix` files if they need OP-TEE within their containers.

**Example from `spv-enclave/pup.nix`:**

```nix
{
  pupEnclave = true;
  imports = [ (pkgs.nixosModules.tee-supplicant) ];
  
  services.tee-supplicant = {
    enable = true;
    trustedApplications = [
      # Same TAs as system level
      "${pkgs.optee-os-rockchip-rk3588.devkit}/ta/023f8f1a-292a-432b-8fc4-de8471358067.ta"
      "${pkgs.optee-os-rockchip-rk3588.devkit}/ta/80a4c275-0a47-4905-8285-1486a9771a08.ta"
      "${pkgs.optee-os-rockchip-rk3588.devkit}/ta/f04a0fe7-1f5d-4b9b-abf7-619b85b4ce8c.ta"
      "${pkgs.optee-os-rockchip-rk3588.devkit}/ta/fd02c9da-306c-48c7-a49c-bbd827ae86ee.ta"
      "${libdogecoin."libdogecoin-optee-ta"}/ta/62d95dc0-7fc2-4cb3-a7f3-c13ae4e633c4.ta"
    ];
  };
}
```

**Why:** Pups run in isolated systemd-nspawn containers and need their own tee-supplicant instance to communicate with OP-TEE.

### Configuration Summary

| Component | Level | Configure In | Purpose |
|-----------|-------|--------------|---------|
| DKM | System | `Dogebox-WG/os` → `nix/dbx/dkm.nix` | DKM service uses OP-TEE |
| Pups | Container | Each pup's `pup.nix` | Pup uses OP-TEE in container |

Both can coexist without conflict because they operate at different isolation levels.

### Trusted Applications

- **Standard OP-TEE TAs**: Platform-specific TAs from optee-os-rockchip-rk3588
- **libdogecoin TA** (UUID: 62d95dc0-7fc2-4cb3-a7f3-c13ae4e633c4): The secure enclave for DKM/pup operations

### Platform Support

This configuration is specific to:
- **Hardware**: RK3588-based devices (e.g., NanoPC-T6)
- **OS**: Dogebox OS (NixOS-based)
- **OP-TEE**: ARM TrustZone implementation for RK3588

For other platforms (x86_64 with Intel SGX, etc.), adjust the trusted applications accordingly.

## References

- Example usage: https://github.com/edtubbs/pups/blob/spv-enclave/spv_enclave/pup.nix
- libdogecoin enclave docs: https://github.com/dogecoinfoundation/libdogecoin/blob/0.1.5-dev/doc/enclaves.md
- libdogecoin package: https://github.com/Dogebox-WG/dogebox-nur-packages/blob/main/pkgs/libdogecoin/default.nix
- Dogebox OS repository: https://github.com/Dogebox-WG/os
