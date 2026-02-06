package enclave

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

var (
	ErrEnclaveNotAvailable  = errors.New("optee_libdogecoin not available")
	ErrEnclaveGenerate      = errors.New("failed to generate mnemonic in enclave")
	ErrEnclaveRead          = errors.New("failed to read from enclave")
	ErrMnemonicNotFound     = errors.New("mnemonic not found in enclave output")
	ErrInvalidEnclaveOutput = errors.New("invalid enclave output format")
)

// OpteeTool represents the optee_libdogecoin CLI tool
type OpteeTool struct {
	binPath string
}

// NewOpteeTool creates a new OP-TEE tool interface
// The binPath defaults to "optee_libdogecoin" in PATH if not specified
func NewOpteeTool(binPath string) (*OpteeTool, error) {
	if binPath == "" {
		binPath = "optee_libdogecoin"
	}
	
	// Check if the tool is available
	_, err := exec.LookPath(binPath)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrEnclaveNotAvailable, err)
	}
	
	return &OpteeTool{binPath: binPath}, nil
}

// GenerateMnemonic generates a new mnemonic in the enclave
// Uses the -c generate_mnemonic command
// The -p flag provides the password for the mnemonic seedphrase
// The -f flag with "delegate" enables delegation features
func (t *OpteeTool) GenerateMnemonic(password string) ([]string, error) {
	cmd := exec.Command(t.binPath, "-c", "generate_mnemonic", "-p", password, "-f", "delegate")
	
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	
	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("%w: %v, stderr: %s", ErrEnclaveGenerate, err, stderr.String())
	}
	
	// Parse the output to extract the mnemonic
	// Expected format: "Mnemonic generated: word1 word2 word3 ..."
	output := stdout.String()
	mnemonic, err := extractMnemonicFromOutput(output)
	if err != nil {
		return nil, err
	}
	
	return mnemonic, nil
}

// GenerateExtendedPublicKey generates an extended public key from the enclave
// If customPath is provided, it will be used instead via -h flag
// Otherwise uses BIP44 path: m/44'/3'/account'/changeLevel
// The -p flag provides the password for authentication
func (t *OpteeTool) GenerateExtendedPublicKey(account, changeLevel int, password string, customPath string) (string, error) {
	var cmd *exec.Cmd
	
	if customPath != "" {
		// Use custom path via -h flag
		cmd = exec.Command(t.binPath, 
			"-c", "generate_extended_public_key",
			"-h", customPath,
			"-p", password)
	} else {
		// Use BIP44 parameters
		cmd = exec.Command(t.binPath, 
			"-c", "generate_extended_public_key",
			"-o", fmt.Sprintf("%d", account),
			"-l", fmt.Sprintf("%d", changeLevel),
			"-p", password)
	}
	
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("failed to generate extended public key: %v, stderr: %s", err, stderr.String())
	}
	
	// Parse the output to extract the extended public key
	// Expected format: "Extended public key generated: xpub..."
	output := stdout.String()
	xpub, err := extractExtendedPublicKeyFromOutput(output)
	if err != nil {
		return "", err
	}
	
	return xpub, nil
}

// GenerateAddress generates a Dogecoin address from the enclave
// account, changeLevel, and addressIndex specify the BIP44 derivation path
// Uses: m/44'/3'/account'/changeLevel/addressIndex
// If customPath is provided, it will be used instead via -h flag
// The -p flag provides the password for authentication
func (t *OpteeTool) GenerateAddress(account, changeLevel, addressIndex int, password string, customPath string) (string, error) {
	var cmd *exec.Cmd
	
	if customPath != "" {
		// Use custom path via -h flag
		cmd = exec.Command(t.binPath, 
			"-c", "generate_address",
			"-h", customPath,
			"-p", password)
	} else {
		// Use BIP44 parameters
		cmd = exec.Command(t.binPath, 
			"-c", "generate_address",
			"-o", fmt.Sprintf("%d", account),
			"-l", fmt.Sprintf("%d", changeLevel),
			"-i", fmt.Sprintf("%d", addressIndex),
			"-p", password)
	}
	
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("failed to generate address: %v, stderr: %s", err, stderr.String())
	}
	
	// Parse the output to extract the address
	// Expected format: "Address generated: D..."
	output := stdout.String()
	address, err := extractAddressFromOutput(output)
	if err != nil {
		return "", err
	}
	
	return address, nil
}

// HasMnemonic checks if a mnemonic is stored in the enclave
// This attempts to generate the extended public key from the master key path "m"
// to verify that the enclave has a mnemonic that matches DKM's expected derivation
func (t *OpteeTool) HasMnemonic(password string) bool {
	// Use master key path "m" to check for mnemonic existence
	// This aligns with how DKM derives the master key
	_, err := t.GenerateExtendedPublicKey(0, 0, password, "m")
	return err == nil
}

// DelegateKey delegates account keys in the enclave
// Uses custom path for DKM's delegate namespace: m/1000'/2'/N'
// The -d flag provides the delegate password
func (t *OpteeTool) DelegateKey(account int, delegatePassword, password, customPath string) error {
	var cmd *exec.Cmd
	
	if customPath != "" {
		// Use custom path via -h flag (e.g., "m/1000'/2'/0'")
		cmd = exec.Command(t.binPath,
			"-c", "delegate_key",
			"-o", fmt.Sprintf("%d", account),
			"-d", delegatePassword,
			"-h", customPath,
			"-p", password)
	} else {
		// Use account number only
		cmd = exec.Command(t.binPath,
			"-c", "delegate_key",
			"-o", fmt.Sprintf("%d", account),
			"-d", delegatePassword,
			"-p", password)
	}
	
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to delegate key: %v, stderr: %s", err, stderr.String())
	}
	
	return nil
}

// ExportDelegateKey exports delegated account keys from the enclave
// Uses the delegate password to export keys
func (t *OpteeTool) ExportDelegateKey(account int, delegatePassword string) (string, error) {
	cmd := exec.Command(t.binPath,
		"-c", "export_delegate_key",
		"-o", fmt.Sprintf("%d", account),
		"-d", delegatePassword)
	
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("failed to export delegate key: %v, stderr: %s", err, stderr.String())
	}
	
	// Return the full output as it contains the exported key
	return stdout.String(), nil
}

// extractExtendedPublicKeyFromOutput parses the extended public key from optee_libdogecoin output
func extractExtendedPublicKeyFromOutput(output string) (string, error) {
	// Look for "Extended public key generated:" or similar pattern
	re := regexp.MustCompile(`(?i)extended\s+public\s+key\s+(?:generated)?:?\s*(\S+)`)
	matches := re.FindStringSubmatch(output)
	
	if len(matches) < 2 {
		return "", fmt.Errorf("%w: no extended public key found in output", ErrInvalidEnclaveOutput)
	}
	
	xpub := strings.TrimSpace(matches[1])
	return xpub, nil
}

// extractMnemonicFromOutput parses the mnemonic from optee_libdogecoin output
func extractMnemonicFromOutput(output string) ([]string, error) {
	// Look for "Mnemonic generated:" pattern
	re := regexp.MustCompile(`Mnemonic generated:\s*(.+)`)
	matches := re.FindStringSubmatch(output)
	
	if len(matches) < 2 {
		return nil, fmt.Errorf("%w: no mnemonic found in output", ErrMnemonicNotFound)
	}
	
	// Split the mnemonic into words
	mnemonicStr := strings.TrimSpace(matches[1])
	words := strings.Fields(mnemonicStr)
	
	if len(words) < 12 {
		return nil, fmt.Errorf("%w: mnemonic has too few words (%d)", ErrInvalidEnclaveOutput, len(words))
	}
	
	return words, nil
}

// extractAddressFromOutput parses the address from optee_libdogecoin output
func extractAddressFromOutput(output string) (string, error) {
	// Look for "Address generated:" pattern
	re := regexp.MustCompile(`Address generated:\s*(\S+)`)
	matches := re.FindStringSubmatch(output)
	
	if len(matches) < 2 {
		return "", fmt.Errorf("%w: no address found in output", ErrInvalidEnclaveOutput)
	}
	
	address := strings.TrimSpace(matches[1])
	return address, nil
}
