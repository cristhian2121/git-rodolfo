package domain

// SSHKeyCandidate is a private key found while scanning a directory for
// existing SSH keys to offer in "account add"'s selector (RF-40), instead
// of forcing the user to type a path by hand.
type SSHKeyCandidate struct {
	PrivateKeyPath string
	PublicKeyPath  string
	Fingerprint    string
}
