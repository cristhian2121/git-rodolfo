package cli

import (
	"fmt"

	"github.com/lean-tech/git-rodolfo/internal/cli/prompt"
)

// runAccountRemove implements "git rodolfo account remove [account]
// [--delete-key]" (RF-05, RF-08, §13.5): two independent confirmations —
// the profile first, the key files second (and only if the profile
// removal was confirmed).
func runAccountRemove(deps Deps, flags GlobalFlags, args []string) int {
	var deleteKey bool
	var positional []string
	for _, a := range args {
		if a == "--delete-key" {
			deleteKey = true
			continue
		}
		positional = append(positional, a)
	}
	if len(positional) > 1 {
		fmt.Fprintf(deps.Stderr, "git-rodolfo: \"account remove\" takes at most one account id (got %d)\n", len(positional))
		return 1
	}

	var session *prompt.Session
	if !flags.NonInteractive {
		session = prompt.NewSession(deps.Stdout, deps.Stdin)
	}

	var id string
	if len(positional) == 1 {
		id = positional[0]
	} else if flags.NonInteractive {
		fmt.Fprintln(deps.Stderr, "git-rodolfo: \"account remove\" requires an account id in --non-interactive mode")
		return 1
	} else {
		chosen, code := selectAccountInteractively(deps, session, "Remove which account?")
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

	if flags.NonInteractive {
		if !flags.Yes {
			fmt.Fprintln(deps.Stderr, "git-rodolfo: \"account remove\" requires --yes in --non-interactive mode")
			return 1
		}
	} else {
		fmt.Fprintf(deps.Stdout, "Remove account %q from Git Rodolfo?\n\n", account.DisplayName)
		fmt.Fprintln(deps.Stdout, "This removes the local profile only.")
		fmt.Fprintln(deps.Stdout, "Your GitHub account and your repositories are not affected.")
		fmt.Fprintln(deps.Stdout)
		confirmed, _ := session.Confirm("Confirm?", false)
		if !confirmed {
			fmt.Fprintln(deps.Stdout, "Aborted. No changes were made.")
			return 0
		}
	}

	removed, err := deps.Remove.RemoveProfile(id)
	if err != nil {
		printAccountKeyError(deps, err)
		return 1
	}
	fmt.Fprintln(deps.Stdout, "\n✓ Account removed.")

	if removed.PrivateKeyPath == "" {
		return 0
	}

	stillUsed, err := deps.Remove.KeyStillUsedByAnotherAccount(removed)
	if err != nil {
		fmt.Fprintf(deps.Stderr, "git-rodolfo: %v\n", err)
		return 1
	}
	if stillUsed {
		fmt.Fprintf(deps.Stdout, "\nThe key %s is still used by another registered account; it will not be deleted.\n", removed.PrivateKeyPath)
		return 0
	}

	if flags.NonInteractive {
		if !deleteKey {
			return 0
		}
		return deleteRemovedAccountKey(deps, removed.PrivateKeyPath)
	}

	fmt.Fprintln(deps.Stdout, "\nAlso delete this account's SSH key files?")
	fmt.Fprintln(deps.Stdout)
	fmt.Fprintf(deps.Stdout, "  %s\n  %s.pub\n\n", removed.PrivateKeyPath, removed.PrivateKeyPath)
	fmt.Fprintln(deps.Stdout, "⚠ This cannot be undone.")
	fmt.Fprintln(deps.Stdout, "  Other applications may be using this key — servers, deploys,")
	fmt.Fprintln(deps.Stdout, "  other tools. Any of them would stop working.")
	fmt.Fprintln(deps.Stdout)
	fmt.Fprintln(deps.Stdout, "  The key is still registered on GitHub. Deleting these files does")
	fmt.Fprintln(deps.Stdout, "  not revoke it there; remove it from GitHub SSH settings as well.")
	fmt.Fprintln(deps.Stdout)
	confirmed, _ := session.Confirm("Delete the key files?", false)
	if !confirmed {
		return 0
	}
	return deleteRemovedAccountKey(deps, removed.PrivateKeyPath)
}

func deleteRemovedAccountKey(deps Deps, privateKeyPath string) int {
	if err := deps.SSH.DeleteKey(privateKeyPath); err != nil {
		fmt.Fprintf(deps.Stderr, "git-rodolfo: %v\n", err)
		return 1
	}
	fmt.Fprintln(deps.Stdout, "✓ Key files deleted.")
	return 0
}
