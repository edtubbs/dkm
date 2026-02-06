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
func (t *OpteeTool) GenerateMnemonic(password string) ([]string, error) {
	cmd := exec.Command(t.binPath, "-c", "generate_mnemonic", "-p", password)
	
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

// GenerateAddress generates a Dogecoin address from the enclave
// account, changeLevel, and addressIndex specify the BIP44 derivation path
// Uses: m/44'/3'/account'/changeLevel/addressIndex
// The -p flag provides the password for authentication
func (t *OpteeTool) GenerateAddress(account, changeLevel, addressIndex int, password string) (string, error) {
	cmd := exec.Command(t.binPath, 
		"-c", "generate_address",
		"-o", fmt.Sprintf("%d", account),
		"-l", fmt.Sprintf("%d", changeLevel),
		"-i", fmt.Sprintf("%d", addressIndex),
		"-p", password)
	
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
// This attempts to generate an address; if successful, mnemonic exists
// A dummy password is used since we're just checking for existence
func (t *OpteeTool) HasMnemonic(password string) bool {
	_, err := t.GenerateAddress(0, 0, 0, password)
	return err == nil
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
