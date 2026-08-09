package app

import "github.com/lean-tech/git-rodolfo/internal/domain"

// SSHKeyOption is one entry in "account add"'s key selector: a scanned
// candidate annotated with whether another registered account already
// claims it (RF-40, RF-41).
type SSHKeyOption struct {
	domain.SSHKeyCandidate
	// InUseBy is the DisplayName of the account already using this key's
	// fingerprint, or "" if the key is free to associate with a new one.
	InUseBy string
}

// ScanAvailableKeys scans dir for existing SSH keys (RF-40) and annotates
// each one with the account already using it, if any (RF-41, reusing the
// same fingerprint check RF-07 already relies on) — so the caller can show
// them all, marking the unavailable ones instead of hiding them.
func ScanAvailableKeys(ssh SSHClient, accounts AccountRepository, dir string) ([]SSHKeyOption, error) {
	candidates, err := ssh.ScanKeys(dir)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, nil
	}

	// One List() covers every candidate's lookup, instead of a separate
	// FindByFingerprint call per candidate re-reading the whole config
	// file from disk each time (ConfigRepository has no other way to
	// look up by fingerprint without a full read+parse).
	all, err := accounts.List()
	if err != nil {
		return nil, err
	}
	ownerByFingerprint := make(map[string]string, len(all))
	for _, a := range all {
		if a.PublicKeyFingerprint != "" {
			ownerByFingerprint[a.PublicKeyFingerprint] = a.DisplayName
		}
	}

	options := make([]SSHKeyOption, len(candidates))
	for i, c := range candidates {
		options[i] = SSHKeyOption{SSHKeyCandidate: c, InUseBy: ownerByFingerprint[c.Fingerprint]}
	}
	return options, nil
}
