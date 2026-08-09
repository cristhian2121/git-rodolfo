package app

import (
	"errors"
	"strings"
)

// ErrUnsafeKeyPath is returned when a key path contains a single quote.
// SSHCommandMechanism embeds the path in a single-quoted shell string
// (core.sshCommand — see sshCommandFor in identity_mechanism.go), which
// Git later runs through a shell on every SSH operation for the
// repository. Inside single quotes, POSIX shells treat everything
// literally except another single quote — so a quote in the path is the
// one character that can break out of it and inject arbitrary shell
// commands that persist in .git/config. Rejecting it here, before the
// path is ever saved to an account, closes that off at the source.
var ErrUnsafeKeyPath = errors.New("key path must not contain a single quote (')")

func validateKeyPath(path string) error {
	if strings.ContainsRune(path, '\'') {
		return ErrUnsafeKeyPath
	}
	return nil
}
