# Deployment Recommendations for Dogebox OS

This document provides recommendations for deploying DKM with OP-TEE support in Dogebox OS.

## Issue: Installation Timing

### Problem
During OS installation, DKM may start before `optee_libdogecoin` and `tee-supplicant` are fully installed. This can result in:
- Confusing error messages during installation
- Race conditions between service startup and package installation
- Mnemonic creation happening before enclave is available

### Solution Options

## Option 1: Environment Variable During Installation (Recommended)

Set `DKM_SKIP_OPTEE=1` during the installation/setup phase:

**In `nix/dbx/dkm.nix` (Dogebox-WG/os):**

```nix
{ config, pkgs, lib, ... }:
{
  systemd.services.dkm = {
    description = "Doge Key Manager";
    wantedBy = [ "multi-user.target" ];
    
    # Set environment variable during installation
    environment = {
      DKM_SKIP_OPTEE = lib.mkIf (config.system.build.isInstalling or false) "1";
    };
    
    serviceConfig = {
      ExecStart = "${pkgs.dkm}/bin/dkm --dir /var/lib/dkm";
      # ... other config
    };
  };
  
  # Enable tee-supplicant for DKM's OP-TEE integration
  services.tee-supplicant = {
    enable = true;
    trustedApplications = [
      # ... TAs ...
    ];
  };
}
```

After installation completes, remove or unset `DKM_SKIP_OPTEE` to enable enclave.

## Option 2: Systemd Service Dependencies

Ensure DKM starts after tee-supplicant and required packages are installed:

**In `nix/dbx/dkm.nix`:**

```nix
systemd.services.dkm = {
  description = "Doge Key Manager";
  wantedBy = [ "multi-user.target" ];
  
  # Start after tee-supplicant
  after = [ "tee-supplicant.service" ];
  requires = [ "tee-supplicant.service" ];
  
  # Ensure optee_libdogecoin is in PATH
  path = [ pkgs.libdogecoin."libdogecoin-optee-host" ];
  
  serviceConfig = {
    ExecStart = "${pkgs.dkm}/bin/dkm --dir /var/lib/dkm";
    # ... other config
  };
};
```

**Note**: This ensures proper ordering but doesn't solve the issue if packages aren't installed yet.

## Option 3: Include in Base OS Image (Best Long-term)

Add OP-TEE components to the base OS installation image:

**In base system configuration:**

```nix
{ config, pkgs, ... }:
{
  # Include OP-TEE components in base image
  environment.systemPackages = [
    pkgs.optee-os-rockchip-rk3588.devkit
    pkgs.libdogecoin."libdogecoin-optee-host"
    pkgs.libdogecoin."libdogecoin-optee-ta"
  ];
  
  # Enable tee-supplicant by default
  services.tee-supplicant = {
    enable = true;
    trustedApplications = [
      "${pkgs.optee-os-rockchip-rk3588.devkit}/ta/023f8f1a-292a-432b-8fc4-de8471358067.ta"
      "${pkgs.optee-os-rockchip-rk3588.devkit}/ta/80a4c275-0a47-4905-8285-1486a9771a08.ta"
      "${pkgs.optee-os-rockchip-rk3588.devkit}/ta/f04a0fe7-1f5d-4b9b-abf7-619b85b4ce8c.ta"
      "${pkgs.optee-os-rockchip-rk3588.devkit}/ta/fd02c9da-306c-48c7-a49c-bbd827ae86ee.ta"
      "${pkgs.libdogecoin."libdogecoin-optee-ta"}/ta/62d95dc0-7fc2-4cb3-a7f3-c13ae4e633c4.ta"
    ];
  };
}
```

**Benefits**:
- No race conditions
- OP-TEE always available from first boot
- No installation timing issues
- Clean user experience

**Trade-offs**:
- Increases base image size
- Not all hardware supports OP-TEE
- May need conditional inclusion based on hardware

## Option 4: Defer Mnemonic Creation (User Workflow)

Instead of creating the mnemonic during installation, defer it to a post-installation setup step:

1. **Installation Phase**: Install OS and all packages including DKM
2. **First Boot**: System boots, all services start properly
3. **Setup Wizard**: User completes setup wizard
4. **Mnemonic Creation**: After setup completes, user creates DKM mnemonic
   - At this point, tee-supplicant and optee_libdogecoin are installed
   - Enclave is available and will be used

This is the approach suggested by the user: "maybe it can wait until after initial setup to do the dkm mnemonic creation"

**Implementation**:
- Don't call `/create` API during installation
- Add setup step that creates mnemonic after installation completes
- DKM service starts cleanly without errors

## Recommended Approach

**Short-term**: Combine Option 1 and Option 4
- Use `DKM_SKIP_OPTEE=1` during installation to suppress messages
- Defer mnemonic creation to post-installation setup

**Long-term**: Option 3
- Include OP-TEE components in base OS image for supported hardware
- Use conditional configuration for hardware without OP-TEE support

## Implementation in Dogebox-WG/os

See the branch: https://github.com/Dogebox-WG/os/compare/main...edtubbs:dogebox:copilot/update-dkm-optee-integration

Recommended changes:

1. **Add systemd dependencies** (Option 2)
2. **Set DKM_SKIP_OPTEE during installation** (Option 1)
3. **Consider including in base image** for supported platforms (Option 3)
4. **Defer mnemonic creation** in setup workflow (Option 4)

## Testing

To test the installation behavior:

```bash
# Simulate installation phase
DKM_SKIP_OPTEE=1 dkm --dir /tmp/test-dkm

# Should start without any enclave-related messages
# Create key after "installation" completes:
curl -X POST http://localhost:8089/create -d '{"password":"test123"}' -H "Content-Type: application/json"

# Verify it falls back to local generation silently
```

With OP-TEE available:

```bash
# Normal operation (no DKM_SKIP_OPTEE)
dkm --dir /tmp/test-dkm

# Create key - should use enclave if available
curl -X POST http://localhost:8089/create -d '{"password":"test123"}' -H "Content-Type: application/json"
```

## References

- DKM Repository: https://github.com/Dogebox-WG/dkm
- OS Repository: https://github.com/Dogebox-WG/os
- libdogecoin OP-TEE: https://github.com/dogecoinfoundation/libdogecoin
