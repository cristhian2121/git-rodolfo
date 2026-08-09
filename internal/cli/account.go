package cli

import (
	"errors"
	"fmt"
	"time"

	"github.com/lean-tech/git-rodolfo/internal/app"
	"github.com/lean-tech/git-rodolfo/internal/cli/prompt"
	"github.com/lean-tech/git-rodolfo/internal/domain"
)

// runAccount dispatches "git rodolfo account <subcommand>".
func runAccount(deps Deps, flags GlobalFlags, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(deps.Stderr, "git-rodolfo: \"account\" requires a subcommand, e.g. \"account show <account>\"")
		return 1
	}

	switch args[0] {
	case "show":
		return runAccountShow(deps, args[1:])
	case "add":
		return runAccountAdd(deps, flags, args[1:])
	case "edit":
		return runAccountEdit(deps, flags, args[1:])
	case "remove":
		return runAccountRemove(deps, flags, args[1:])
	default:
		fmt.Fprintf(deps.Stderr, "git-rodolfo: unknown \"account\" subcommand %q\n", args[0])
		return 1
	}
}

// selectAccountInteractively shows an arrow-key selector over every
// registered account and returns the chosen one's ID. It's shared by
// "account edit"/"account remove" when no account id is given. It takes an
// existing session rather than creating its own: a second bufio-buffered
// Session on the same Stdin can silently swallow input meant for whatever
// prompt comes after it.
func selectAccountInteractively(deps Deps, session *prompt.Session, label string) (string, int) {
	accounts, err := deps.Accounts.List()
	if err != nil {
		fmt.Fprintf(deps.Stderr, "git-rodolfo: %v\n", err)
		return "", 1
	}
	if len(accounts) == 0 {
		fmt.Fprintln(deps.Stderr, "git-rodolfo: no accounts registered. Run \"git rodolfo account add\" first.")
		return "", 1
	}

	labels := make([]string, len(accounts))
	for i, a := range accounts {
		labels[i] = a.DisplayName
	}

	choice, err := session.Select(label, labels)
	if errors.Is(err, prompt.ErrCanceled) {
		fmt.Fprintln(deps.Stderr, "Canceled.")
		return "", 1
	}
	if err != nil {
		fmt.Fprintf(deps.Stderr, "git-rodolfo: %v\n", err)
		return "", 1
	}
	return accounts[choice].ID, 0
}

// printAccountKeyError renders the §13.2/§21 duplicate-key message shared
// by "account add" and "account edit", or a generic error line otherwise.
func printAccountKeyError(deps Deps, err error) {
	var dup *app.KeyAlreadyInUseError
	if errors.As(err, &dup) {
		fmt.Fprintf(deps.Stderr, "The key %s is already used by the account %q.\n\n", dup.KeyPath, dup.OtherAccountName)
		fmt.Fprintln(deps.Stderr, "GitHub does not allow the same public key on two accounts.")
		fmt.Fprintf(deps.Stderr, "Generate a new key for %q.\n", dup.RequestedForName)
		return
	}
	if errors.Is(err, app.ErrAccountNotFound) {
		fmt.Fprintln(deps.Stderr, "git-rodolfo: account not found. Run \"git rodolfo accounts\" to list registered accounts.")
		return
	}
	fmt.Fprintf(deps.Stderr, "git-rodolfo: %v\n", err)
}

// runAccountShow implements "git rodolfo account show <account>" (RF-03,
// PRD §13.4).
func runAccountShow(deps Deps, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(deps.Stderr, "git-rodolfo: \"account show\" requires an account id, e.g. \"account show lean-tech\"")
		return 1
	}
	if len(args) > 1 {
		fmt.Fprintf(deps.Stderr, "git-rodolfo: \"account show\" takes a single account id (got %d)\n", len(args))
		return 1
	}

	id := args[0]
	account, err := deps.Accounts.Get(id)
	if errors.Is(err, app.ErrAccountNotFound) {
		fmt.Fprintf(deps.Stderr, "git-rodolfo: no account named %q. Run \"git rodolfo accounts\" to list registered accounts.\n", id)
		return 1
	}
	if err != nil {
		fmt.Fprintf(deps.Stderr, "git-rodolfo: %v\n", err)
		return 1
	}

	fmt.Fprint(deps.Stdout, formatAccountShow(account, deps.Now()))
	return 0
}

// formatAccountShow renders "account show" output matching PRD §13.4. It
// never includes the private key's contents, only its path.
func formatAccountShow(a domain.Account, now time.Time) string {
	return fmt.Sprintf(
		"Account: %s\nProvider: %s\nUsername: %s\nCommit name: %s\nCommit email: %s\nSSH key: %s\nAuthentication: %s\n",
		a.DisplayName,
		providerDisplayName(a.Provider),
		a.ProviderUsername,
		a.GitName,
		a.GitEmail,
		a.PrivateKeyPath,
		authenticationDetail(a, now),
	)
}
