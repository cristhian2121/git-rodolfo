package app

import (
	"errors"
	"fmt"
	"strings"

	"github.com/lean-tech/git-rodolfo/internal/domain"
)

// OperationContext is what ErrorTranslator needs to explain a failure:
// which account was used, and against which remote.
type OperationContext struct {
	Account   domain.Account
	RemoteURL string
}

// ErrorTranslator implements RF-21 / §13.9 / §21: Git and SSH errors are
// shown as-is (principle 8) except the ambiguous ones documented in §4.5.
// "Permission denied (publickey)" is unambiguous already, so it isn't
// rewritten — only a suggested command is appended.
type ErrorTranslator struct {
	accounts AccountRepository
}

// NewErrorTranslator builds an ErrorTranslator.
func NewErrorTranslator(accounts AccountRepository) *ErrorTranslator {
	return &ErrorTranslator{accounts: accounts}
}

// Translate returns (message, true) when err's underlying Git output
// matches one of §4.5's ambiguous cases, or ("", false) when it should be
// shown unmodified — the default for "any other Git error" (principle 8).
func (t *ErrorTranslator) Translate(err error, ctx OperationContext) (string, bool) {
	var gitErr *GitCommandError
	if !errors.As(err, &gitErr) {
		return "", false
	}
	output := gitErr.Output

	switch {
	case strings.Contains(output, "Repository not found"):
		return t.translateRepositoryNotFound(output, ctx), true
	case strings.Contains(output, "Permission denied (publickey)"):
		return output + "\n\nRun:\ngit rodolfo doctor", true
	default:
		return "", false
	}
}

// translateRepositoryNotFound is CU-07 / §21's main case: GitHub gives the
// same error for "doesn't exist" and "no access," which in the
// multi-account scenario is usually the second — but the message asserts
// the first.
func (t *ErrorTranslator) translateRepositoryNotFound(output string, ctx OperationContext) string {
	var b strings.Builder
	fmt.Fprintln(&b, "GitHub says the repository was not found.")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "This means one of two things:")
	fmt.Fprintln(&b, "  - The repository does not exist, or")
	fmt.Fprintln(&b, "  - The account you are using does not have access to it.")
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "You authenticated as: %s (%s)\n", ctx.Account.ProviderUsername, ctx.Account.DisplayName)

	if u, parseErr := domain.ParseRepositoryURL(ctx.RemoteURL); parseErr == nil {
		fmt.Fprintf(&b, "The repository belongs to: %s\n", u.Owner)

		if suggestion, err := t.accounts.FindByID(strings.ToLower(u.Owner)); err == nil && suggestion != nil && suggestion.ID != ctx.Account.ID {
			fmt.Fprintln(&b)
			fmt.Fprintf(&b, "You have an account for that organization: %q\n", suggestion.DisplayName)
			fmt.Fprintln(&b)
			fmt.Fprintln(&b, "Try:")
			fmt.Fprintf(&b, "git rodolfo clone %s --account %s\n", ctx.RemoteURL, suggestion.ID)
		}
	}

	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Original error:")
	fmt.Fprintln(&b, output)
	return strings.TrimRight(b.String(), "\n")
}
