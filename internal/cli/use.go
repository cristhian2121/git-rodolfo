package cli

import (
	"fmt"

	"github.com/lean-tech/git-rodolfo/internal/cli/prompt"
	"github.com/lean-tech/git-rodolfo/internal/domain"
)

// runUse implements "git rodolfo use [account] [--clear]" (RF-17, RF-18,
// RF-19, RF-20, §13.7).
func runUse(deps Deps, flags GlobalFlags, args []string) int {
	var clear bool
	var positional []string
	for _, a := range args {
		if a == "--clear" {
			clear = true
			continue
		}
		positional = append(positional, a)
	}
	if len(positional) > 1 {
		fmt.Fprintf(deps.Stderr, "git-rodolfo: \"use\" takes at most one account id (got %d)\n", len(positional))
		return 1
	}

	svc, _, code := bindRepositoryIdentity(deps)
	if code != 0 {
		return code
	}

	if clear {
		if err := svc.Clear(); err != nil {
			fmt.Fprintf(deps.Stderr, "git-rodolfo: %v\n", err)
			return 1
		}
		fmt.Fprintln(deps.Stdout, "Repository identity cleared.")
		return 0
	}

	var id string
	if len(positional) == 1 {
		id = positional[0]
	} else if flags.NonInteractive {
		fmt.Fprintln(deps.Stderr, "git-rodolfo: \"use\" requires an account id in --non-interactive mode")
		return 1
	} else {
		session := prompt.NewSession(deps.Stdout, deps.Stdin)
		chosen, code := selectAccountInteractively(deps, session, "Select an account:")
		if code != 0 {
			return code
		}
		id = chosen
	}

	account, err := deps.Accounts.Get(id)
	if err != nil {
		printAccountKeyError(deps, err)
		return 1
	}

	if err := svc.Use(account); err != nil {
		fmt.Fprintf(deps.Stderr, "git-rodolfo: %v\n", err)
		return 1
	}

	remoteURL, hasRemote, err := svc.RemoteURL()
	if err != nil {
		fmt.Fprintf(deps.Stderr, "git-rodolfo: %v\n", err)
		return 1
	}

	fmt.Fprintln(deps.Stdout, "Repository identity updated.")
	fmt.Fprintln(deps.Stdout)
	fmt.Fprintf(deps.Stdout, "Account: %s\n", account.DisplayName)
	fmt.Fprintf(deps.Stdout, "Commit email: %s\n", account.GitEmail)

	if !hasRemote {
		fmt.Fprintln(deps.Stdout, "Remote: none — identity will apply when you add one.")
		return 0
	}
	if account.PrivateKeyPath != "" {
		fmt.Fprintf(deps.Stdout, "SSH key: %s\n", account.PrivateKeyPath)
	}
	if u, err := domain.ParseRepositoryURL(remoteURL); err == nil && u.Host != "github.com" {
		fmt.Fprintf(deps.Stdout, "Remote: %s (unchanged) — warning: host %q is not supported by Git Rodolfo\n", remoteURL, u.Host)
	} else {
		fmt.Fprintf(deps.Stdout, "Remote: %s (unchanged)\n", remoteURL)
	}
	return 0
}
