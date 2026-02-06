# Flag Fix Summary

## Problem

The initial implementation incorrectly used the `-z` flag, assuming it skipped YubiKey authentication. However, according to the libdogecoin documentation:

- **`-z` flag ENABLES YubiKey authentication** (not skips it)
- **`-p` flag provides the password** for the mnemonic
- **`-s` flag is for shared secret** (TOTP, not needed for DKM)

## Changes Made

### 1. Updated `OpteeTool` Methods

**Before:**
```go
func (t *OpteeTool) GenerateMnemonic() ([]string, error)
func (t *OpteeTool) GenerateAddress(account, changeLevel, addressIndex int) (string, error)
func (t *OpteeTool) HasMnemonic() bool
```

**After:**
```go
func (t *OpteeTool) GenerateMnemonic(password string) ([]string, error)
func (t *OpteeTool) GenerateAddress(account, changeLevel, addressIndex int, password string) (string, error)
func (t *OpteeTool) HasMnemonic(password string) bool
```

### 2. Updated CLI Invocations

**Before:**
```bash
optee_libdogecoin -c generate_mnemonic -z
optee_libdogecoin -c generate_address -z -o 0 -l 0 -i 0
```

**After:**
```bash
optee_libdogecoin -c generate_mnemonic -p <password>
optee_libdogecoin -c generate_address -o 0 -l 0 -i 0 -p <password>
```

### 3. Updated Key Manager

Modified `CreateKey()` to pass the user's password to all enclave operations:
- `opteeTool.HasMnemonic(pass)` - check with password
- `opteeTool.GenerateMnemonic(pass)` - generate with password

### 4. Updated Documentation

- README.md - corrected CLI examples and flag descriptions
- docs/OPTEE-INTEGRATION.md - updated interface signatures and examples
- IMPLEMENTATION_SUMMARY.md - corrected implementation details

## Why This Matters

### Security Implications
- The mnemonic in the enclave is now protected by the user's password
- Without `-z`, YubiKey authentication is not required (simpler for DKM)
- The password ensures only authorized access to enclave operations

### Correctness
- Matches the actual libdogecoin CLI interface
- Follows the pattern used in pups/spv_enclave
- Aligns with the documentation at libdogecoin/doc/enclaves.md

## Testing

Build verified successful:
```bash
$ make clean && make
$ ls -lh dkm
-rwxrwxr-x 1 runner runner 14M Feb  6 03:12 dkm
```

## References

From libdogecoin documentation:
- "The `-p` flag is used to provide the password for the mnemonic seedphrase."
- "The `-z` flag is used to enable YubiKey authentication."
- Example: `optee_libdogecoin -c generate_mnemonic -p <password> -z`

The correct usage for DKM (without YubiKey):
- `optee_libdogecoin -c generate_mnemonic -p <password>`
