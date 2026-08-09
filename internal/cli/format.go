package cli

import (
	"fmt"
	"time"

	"github.com/lean-tech/git-rodolfo/internal/domain"
)

// staleAfter is the age past which a "verified" status is no longer shown
// as current (RF-03, PRD §13.4): "verified three months ago" means nothing.
const staleAfter = 7 * 24 * time.Hour

var providerDisplayNames = map[string]string{
	domain.ProviderGitHub: "GitHub",
}

func providerDisplayName(provider string) string {
	if name, ok := providerDisplayNames[provider]; ok {
		return name
	}
	return provider
}

// humanizeAgo renders a non-negative duration as "2 hours ago" style text.
func humanizeAgo(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return pluralize(int(d.Minutes()), "minute") + " ago"
	case d < 24*time.Hour:
		return pluralize(int(d.Hours()), "hour") + " ago"
	default:
		return pluralize(int(d.Hours()/24), "day") + " ago"
	}
}

func pluralize(n int, unit string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", unit)
	}
	return fmt.Sprintf("%d %ss", n, unit)
}

// listStatusText renders the compact status shown in "accounts" (§13.1).
func listStatusText(a domain.Account) string {
	switch a.AuthenticationStatus {
	case domain.AuthenticationStatusVerified:
		return "✓ verified"
	case domain.AuthenticationStatusFailed:
		return "✗ authentication failed"
	default:
		return "○ unverified"
	}
}

// authenticationDetail renders the "Authentication:" line shown in
// "account show" (§13.4): always paired with the status's age, and
// degraded to "not verified recently" past staleAfter.
func authenticationDetail(a domain.Account, now time.Time) string {
	if a.LastVerifiedAt == nil {
		if a.AuthenticationStatus == domain.AuthenticationStatusFailed {
			return "authentication failed"
		}
		return "not verified yet"
	}

	age := now.Sub(*a.LastVerifiedAt)
	if age < 0 {
		age = 0
	}
	ago := humanizeAgo(age)

	switch a.AuthenticationStatus {
	case domain.AuthenticationStatusVerified:
		if age > staleAfter {
			return fmt.Sprintf("not verified recently (last verified %s)", ago)
		}
		return "verified " + ago
	case domain.AuthenticationStatusFailed:
		return "authentication failed " + ago
	default:
		return "not verified yet"
	}
}
