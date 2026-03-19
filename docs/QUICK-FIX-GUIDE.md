# Quick Fix Guide: OP-TEE Integration

## ⚠️ CRITICAL WARNING: Storage Isolation

**Before deploying OP-TEE with DKM, read this:**

### Problem: Shared Storage Overwrites

OP-TEE secure storage is **NOT isolated** between host and containers:
- DKM (host) and pups (containers) access the **same OP-TEE storage location**
- If both generate mnemonics, **they will overwrite each other**
- Container isolation does NOT apply to OP-TEE (hardware/kernel level)

### Required Configuration

**✅ CORRECT: Only ONE component uses OP-TEE mnemonic storage**
- DKM (host) → Uses OP-TEE for mnemonic
- Pups → Use local storage OR derive from DKM

**❌ INCORRECT: Both use OP-TEE**
- DKM generates mnemonic in OP-TEE
- Pup also generates mnemonic in OP-TEE → **OVERWRITES DKM's mnemonic!**

---

## Problem: TEEC_InitializeContext failed with code 0xffff0008

This error means the libdogecoin Trusted Application (TA) is not found on the HOST system.

### Root Cause

**DKM is a system service** running on the HOST, but the libdogecoin TA may only be installed in pup containers (e.g., spv_enclave). The TA **must** be installed on the HOST system for DKM to access it.

## Quick Fix for Dogebox-WG/os

Add to `nix/dbx/dkm.nix`:

```nix
{ config, pkgs, lib, ... }:
let
  # Fetch libdogecoin package
  libdogecoin = pkgs.callPackage (pkgs.fetchurl {
    url = "https://raw.githubusercontent.com/Dogebox-WG/dogebox-nur-packages/refs/heads/main/pkgs/libdogecoin/default.nix";
    sha256 = "sha256-YOUR-HASH-HERE";
  }) {};
in
{
  # CRITICAL: Install TA on HOST system
  environment.systemPackages = [
    libdogecoin."libdogecoin-optee-ta"     # TA must be on HOST
    libdogecoin."libdogecoin-optee-host"   # CLI must be on HOST
  ];
  
  # Enable tee-supplicant on HOST
  services.tee-supplicant = {
    enable = true;
    trustedApplications = [
      # Platform TAs
      "${pkgs.optee-os-rockchip-rk3588.devkit}/ta/023f8f1a-292a-432b-8fc4-de8471358067.ta"
      "${pkgs.optee-os-rockchip-rk3588.devkit}/ta/80a4c275-0a47-4905-8285-1486a9771a08.ta"
      "${pkgs.optee-os-rockchip-rk3588.devkit}/ta/f04a0fe7-1f5d-4b9b-abf7-619b85b4ce8c.ta"
      "${pkgs.optee-os-rockchip-rk3588.devkit}/ta/fd02c9da-306c-48c7-a49c-bbd827ae86ee.ta"
      
      # libdogecoin TA - REQUIRED for DKM
      "${libdogecoin."libdogecoin-optee-ta"}/ta/62d95dc0-7fc2-4cb3-a7f3-c13ae4e633c4.ta"
    ];
  };
  
  # Ensure DKM starts after tee-supplicant
  systemd.services.dkm = {
    after = [ "tee-supplicant.service" ];
    requires = [ "tee-supplicant.service" ];
    
    # Ensure optee_libdogecoin is in DKM's PATH
    path = [ libdogecoin."libdogecoin-optee-host" ];
  };
}
```

## Verification Steps

After applying the fix:

### 1. Check TA is installed on HOST
```bash
ls -la /lib/optee_armtz/62d95dc0-7fc2-4cb3-a7f3-c13ae4e633c4.ta
```
Should exist and be readable.

### 2. Check tee-supplicant is running
```bash
systemctl status tee-supplicant
```
Should show "active (running)".

### 3. Check /dev/tee0 exists
```bash
ls -la /dev/tee*
```
Should show `/dev/tee0` and `/dev/teepriv0`.

### 4. Test DKM with diagnostics
```bash
DKM_DEBUG=1 dkm --dir /tmp/test
```
Should show:
```
=== OP-TEE Diagnostics ===
✓ /dev/tee0 found - OP-TEE device available
✓ tee-supplicant process is running
✓ TA directory /lib/optee_armtz exists
✓ libdogecoin TA found: 62d95dc0-7fc2-4cb3-a7f3-c13ae4e633c4.ta
```

### 5. Test mnemonic creation
```bash
curl -X POST http://localhost:8089/create \
  -H "Content-Type: application/json" \
  -d '{"password":"test123"}'
```
Should create mnemonic without errors.

## Why This Happens

| Component | Level | Where Installed | Why |
|-----------|-------|-----------------|-----|
| DKM | System service | HOST | Manages keys for entire system |
| spv_enclave | Pup container | Container | Isolated wallet app |
| tee-supplicant | System service | HOST | DKM accesses it |
| libdogecoin TA | Trusted App | **HOST** | DKM needs host-level access |

**Key Point**: DKM runs on the HOST and needs HOST-level access to the TA. Installing it only in pup containers doesn't help DKM.

## Common Mistakes

❌ **Installing TA only in pup containers**
- spv_enclave pup has its own tee-supplicant
- But DKM can't access pup's containerized TAs
- Solution: Install on HOST too

❌ **Assuming tee-supplicant is enough**
- tee-supplicant running ≠ TAs installed
- Need both service AND TAs
- Solution: Check /lib/optee_armtz/

❌ **Wrong package on HOST**
- Installing only libdogecoin-optee-host
- This is the CLI, not the TA
- Solution: Install libdogecoin-optee-ta too

❌ **Both DKM and pups using OP-TEE mnemonic storage**
- DKM generates mnemonic in OP-TEE
- Pup also generates mnemonic in OP-TEE
- Result: Pup overwrites DKM's mnemonic!
- Solution: Only DKM should use OP-TEE mnemonic storage

## Configuring Pups to NOT Use OP-TEE Mnemonic Storage

If you have pups (like spv-enclave) that currently use `optee_libdogecoin` for mnemonic generation, you need to reconfigure them to avoid storage conflicts with DKM.

### Option 1: Use Local Storage in Pups

Configure pups to use local mnemonic storage instead of OP-TEE:

```python
# In pup code, use local BIP39 library instead of optee_libdogecoin
from mnemonic import Mnemonic

# Generate locally (NOT in OP-TEE)
mnemo = Mnemonic("english")
mnemonic_phrase = mnemo.generate(strength=256)

# Store in encrypted local file
# (use pup's own encryption, not OP-TEE)
```

### Option 2: Derive Keys from DKM via Delegation

Configure pups to request delegated keys from DKM instead of managing their own mnemonics:

```python
# In pup code, request delegated key from DKM
import requests

# Call DKM API to get a delegated key for this pup
response = requests.post("http://dkm:8089/delegate", json={
    "pup_name": "spv-enclave",
    "purpose": "blockchain_signing"
})

# Use the delegated key (not a full mnemonic)
delegated_key = response.json()["key"]
```

### Option 3: Remove OP-TEE from Pup Configuration

If a pup's `pup.nix` currently includes tee-supplicant configuration, consider whether it actually needs OP-TEE:

```nix
# pup.nix - BEFORE (causes conflicts)
{
  pupEnclave = true;
  imports = [ (pkgs.nixosModules.tee-supplicant) ];
  services.tee-supplicant = {
    enable = true;
    trustedApplications = [ /* libdogecoin TA */ ];
  };
}

# pup.nix - AFTER (no OP-TEE mnemonic storage)
{
  # Remove OP-TEE if not actually needed
  # OR keep OP-TEE but don't use it for mnemonic generation
  pupEnclave = false;
  
  # Use local storage or DKM delegation instead
}
```

**Note**: A pup can still use OP-TEE for OTHER purposes (like signing operations), but should not use `optee_libdogecoin -c generate_mnemonic`.

## During Installation

To avoid confusing errors during OS installation:

```nix
systemd.services.dkm = {
  environment = {
    # Skip enclave checks during installation
    DKM_SKIP_OPTEE = lib.mkIf (config.system.build.isInstalling or false) "1";
  };
};
```

After installation completes, restart DKM to enable enclave.

## Summary: Deployment Checklist

- [ ] **DKM (host)**: Install libdogecoin-optee-ta on HOST
- [ ] **DKM (host)**: Install libdogecoin-optee-host on HOST
- [ ] **DKM (host)**: Enable tee-supplicant on HOST
- [ ] **DKM (host)**: DKM uses OP-TEE for mnemonic storage
- [ ] **Pups**: Do NOT use OP-TEE for mnemonic generation
- [ ] **Pups**: Use local storage OR request delegated keys from DKM
- [ ] **Verify**: Only ONE component generates mnemonics in OP-TEE

## References

- Main docs: [OPTEE-INTEGRATION.md](./OPTEE-INTEGRATION.md)
- Deployment guide: [DEPLOYMENT-RECOMMENDATIONS.md](./DEPLOYMENT-RECOMMENDATIONS.md)
- OS branch: https://github.com/Dogebox-WG/os/compare/main...edtubbs:dogebox:copilot/update-dkm-optee-integration
