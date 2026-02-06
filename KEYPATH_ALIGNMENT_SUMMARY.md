# Custom Keypath Alignment Summary

## Problem

DKM uses specific BIP32 key derivation paths:
- **Master key**: `m` (direct from mnemonic)
- **Pup/Delegate namespace**: `m/1000'/2'/N'` (where N is delegate index)

The initial OP-TEE integration only used BIP44 parameters (`-o`, `-l`, `-i`) which construct standard Dogecoin paths like `m/44'/3'/0'/0/0`. This didn't align with DKM's custom key derivation scheme.

## Solution

Added support for the `-h <custom_path>` flag in libdogecoin optee CLI to specify custom BIP32 key paths that match DKM's derivation.

## Changes Made

### 1. Added `GenerateExtendedPublicKey` Method

New method to derive extended public keys at any BIP32 path:

```go
func (t *OpteeTool) GenerateExtendedPublicKey(account, changeLevel int, password string, customPath string) (string, error)
```

**Usage:**
- When `customPath` is provided (e.g., `"m"`), uses `-h` flag: `optee_libdogecoin -c generate_extended_public_key -h m -p <password>`
- When `customPath` is empty, uses BIP44 parameters: `-o <account> -l <changeLevel>`

**Purpose:**
- Derive master extended public key at path `m`
- Derive pup/delegate namespace keys at path `m/1000'/2'`
- Verify mnemonic matches DKM's expected keys

### 2. Updated `GenerateAddress` Method

Added optional `customPath` parameter:

```go
func (t *OpteeTool) GenerateAddress(account, changeLevel, addressIndex int, password string, customPath string) (string, error)
```

**Usage:**
- When `customPath` is provided, uses `-h` flag
- When `customPath` is empty, uses BIP44 parameters `-o`, `-l`, `-i`

### 3. Updated `HasMnemonic` Method

Now uses master key path for verification:

```go
func (t *OpteeTool) HasMnemonic(password string) bool {
    _, err := t.GenerateExtendedPublicKey(0, 0, password, "m")
    return err == nil
}
```

**Why:**
- Verifies the enclave has a mnemonic by deriving at path `m`
- This matches how DKM derives its master key from the mnemonic
- More accurate check than deriving an arbitrary BIP44 address

### 4. Added Output Parser

New function to extract extended public keys from CLI output:

```go
func extractExtendedPublicKeyFromOutput(output string) (string, error)
```

Parses output like: `"Extended public key generated: xpub..."`

## Key Derivation Alignment

### DKM's Key Paths

**Master Key Storage:**
```go
// From keymgr.go CreateKey()
master, err := doge.Bip32MasterFromSeed(seed, &doge.DogeMainNetChain)
pub := master.GetECPubKey()  // Master key public key
key := []byte(master.EncodeWIF())  // Master key WIF
```

**Pup/Delegate Keys:**
```go
// From keymgr.go CreateDelegate()
// pup namespace: m/1000'/2'/N'
const H = doge.HardenedKey
pupKey, err := master.PrivateCKD([]uint32{H + 1000, H + 2}, true)
child, err = pupKey.PrivateCKD([]uint32{H + keyIndex}, true)
```

### OP-TEE Enclave Calls

**Master Key Verification:**
```bash
optee_libdogecoin -c generate_extended_public_key -h m -p <password>
```

**Pup/Delegate Namespace (future use):**
```bash
optee_libdogecoin -c generate_extended_public_key -h "m/1000'/2'" -p <password>
```

**Specific Delegate Key (future use):**
```bash
optee_libdogecoin -c generate_extended_public_key -h "m/1000'/2'/0'" -p <password>
```

## Benefits

1. **Alignment**: OP-TEE enclave now derives keys at the same paths as DKM
2. **Verification**: `HasMnemonic()` accurately checks for master key existence
3. **Flexibility**: Supports both custom paths and standard BIP44 paths
4. **Future-proof**: Ready for delegate key operations in the enclave
5. **Correctness**: Matches the actual key derivation scheme DKM uses

## CLI Commands Generated

### Before (BIP44 only)
```bash
# Could only do standard BIP44 paths
optee_libdogecoin -c generate_address -o 0 -l 0 -i 0 -p <password>
# Path: m/44'/3'/0'/0/0
```

### After (Custom paths supported)
```bash
# Master key
optee_libdogecoin -c generate_extended_public_key -h m -p <password>

# Pup namespace
optee_libdogecoin -c generate_extended_public_key -h "m/1000'/2'" -p <password>

# Specific delegate
optee_libdogecoin -c generate_extended_public_key -h "m/1000'/2'/0'" -p <password>

# Still supports BIP44
optee_libdogecoin -c generate_address -o 0 -l 0 -i 0 -p <password>
```

## Testing

- ✅ Build successful
- ✅ Code compiles without errors
- ✅ Documentation updated

## Future Enhancements

Potential additions for delegate key management:

1. **Get Delegate Key**: Derive delegate keys from enclave at `m/1000'/2'/N'`
2. **Sign with Delegate**: Sign transactions using delegate keys in enclave
3. **Export Delegate**: Export delegate keys securely

## References

- DKM key derivation: `internal/keymgr/keymgr.go` lines 247-250, 300-302
- libdogecoin CLI docs: https://github.com/dogecoinfoundation/libdogecoin/blob/0.1.5-dev/doc/enclaves.md
- BIP32 spec: https://github.com/bitcoin/bips/blob/master/bip-0032.mediawiki
- BIP44 spec: https://github.com/bitcoin/bips/blob/master/bip-0044.mediawiki
