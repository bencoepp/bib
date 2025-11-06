package secure

import (
	"fmt"
	"os"

	"github.com/charmbracelet/x/term"
)

// ReadPassphrase prompts for a passphrase without echo.
func ReadPassphrase(prompt string) ([]byte, error) {
	fmt.Fprint(os.Stderr, prompt)
	pw, err := term.ReadPassword(uintptr(int(os.Stdin.Fd())))
	fmt.Fprintln(os.Stderr)
	return pw, err
}

// ReadSecondFactor retrieves second factor from environment.
// Recommended: BIBD_SECOND_FACTOR is base64 or raw secret (>=32 bytes random).
func ReadSecondFactor() ([]byte, error) {
	val := os.Getenv("BIBD_SECOND_FACTOR")
	if val == "" {
		return nil, fmt.Errorf("BIBD_SECOND_FACTOR not set")
	}
	return []byte(val), nil
}
