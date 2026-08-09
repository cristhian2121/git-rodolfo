package domain

// ValidationResult is the outcome of checking whether a repository's local
// Git configuration actually matches an account's expected identity
// (PRD §19.2, used by IdentityMechanism.Verify and "git rodolfo current").
type ValidationResult struct {
	OK     bool
	Issues []string
}
