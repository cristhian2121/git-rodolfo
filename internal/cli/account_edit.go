package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/lean-tech/git-rodolfo/internal/app"
	"github.com/lean-tech/git-rodolfo/internal/cli/prompt"
)

// runAccountEdit implements "git rodolfo account edit [account]" (RF-04,
// §13.3).
func runAccountEdit(deps Deps, flags GlobalFlags, args []string) int {
	if flags.NonInteractive {
		return runAccountEditNonInteractive(deps, args)
	}
	return runAccountEditInteractive(deps, args)
}

func runAccountEditNonInteractive(deps Deps, args []string) int {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		fmt.Fprintln(deps.Stderr, "git-rodolfo: \"account edit\" requires an account id in --non-interactive mode")
		return 1
	}
	id := args[0]

	var displayName, commitName, commitEmail, username, key string
	var deleteOldKey bool
	fs := flag.NewFlagSet("account edit", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&displayName, "display-name", "", "new display name")
	fs.StringVar(&commitName, "commit-name", "", "new commit name")
	fs.StringVar(&commitEmail, "commit-email", "", "new commit email")
	fs.StringVar(&username, "username", "", "new provider username")
	fs.StringVar(&key, "key", "", "path to a different existing private key")
	fs.BoolVar(&deleteOldKey, "delete-old-key", false, "delete the previous key's files once --key replaces it")
	if err := fs.Parse(args[1:]); err != nil {
		fmt.Fprintf(deps.Stderr, "git-rodolfo: %v\n", err)
		return 1
	}
	if extra := fs.Args(); len(extra) > 0 {
		fmt.Fprintf(deps.Stderr, "git-rodolfo: unexpected argument %q\n", extra[0])
		return 1
	}
	if displayName == "" && commitName == "" && commitEmail == "" && username == "" && key == "" {
		fmt.Fprintln(deps.Stderr, "git-rodolfo: specify at least one field to change")
		return 1
	}
	if deleteOldKey && key == "" {
		fmt.Fprintln(deps.Stderr, "git-rodolfo: --delete-old-key requires --key")
		return 1
	}

	input := app.AccountEditInput{
		ID:               id,
		DisplayName:      displayName,
		GitName:          commitName,
		GitEmail:         commitEmail,
		ProviderUsername: username,
	}
	if key != "" {
		input.NewKeyPath = expandHome(key)
	}

	result, err := deps.Edit.Edit(input)
	if err != nil {
		printAccountKeyError(deps, err)
		return 1
	}

	fmt.Fprintf(deps.Stdout, "Account %q updated.\n", result.After.DisplayName)
	printAccountEditDiff(deps, result)

	if result.OldKeyPath == "" {
		return 0
	}
	if !deleteOldKey {
		return 0
	}
	if err := deps.SSH.DeleteKey(result.OldKeyPath); err != nil {
		fmt.Fprintf(deps.Stderr, "git-rodolfo: %v\n", err)
		return 1
	}
	fmt.Fprintln(deps.Stdout, "✓ Previous key files deleted.")
	return 0
}

func runAccountEditInteractive(deps Deps, args []string) int {
	session := prompt.NewSession(deps.Stdout, deps.Stdin)

	var id string
	if len(args) > 0 {
		id = args[0]
	} else {
		chosen, code := selectAccountInteractively(deps, session, "Edit which account?")
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
	displayName, _ := session.Text(fmt.Sprintf("Display name [%s]", account.DisplayName))
	commitName, _ := session.Text(fmt.Sprintf("Commit name [%s]", account.GitName))
	commitEmail, _ := session.Text(fmt.Sprintf("Commit email [%s]", account.GitEmail))
	username, _ := session.Text(fmt.Sprintf("Provider username [%s]", account.ProviderUsername))

	input := app.AccountEditInput{
		ID:               id,
		DisplayName:      displayName,
		GitName:          commitName,
		GitEmail:         commitEmail,
		ProviderUsername: username,
	}

	changeKey, _ := session.Confirm("Change the SSH key?", false)
	if changeKey {
		raw, _ := session.Text("Path to the new private key")
		input.NewKeyPath = expandHome(raw)
	}

	if displayName == "" && commitName == "" && commitEmail == "" && username == "" && input.NewKeyPath == "" {
		fmt.Fprintln(deps.Stdout, "Nothing to change.")
		return 0
	}

	result, err := deps.Edit.Preview(input)
	if err != nil {
		printAccountKeyError(deps, err)
		return 1
	}

	fmt.Fprintf(deps.Stdout, "\nUpdate account %q?\n", result.Before.DisplayName)
	printAccountEditDiff(deps, result)
	confirmed, _ := session.Confirm("\nConfirm?", false)
	if !confirmed {
		fmt.Fprintln(deps.Stdout, "Aborted. No changes were made.")
		return 1
	}

	if err := deps.Edit.Commit(result.After); err != nil {
		fmt.Fprintf(deps.Stderr, "git-rodolfo: %v\n", err)
		return 1
	}
	fmt.Fprintln(deps.Stdout, "\n✓ Account updated.")

	if result.OldKeyPath != "" {
		deleteOld, _ := session.Confirm(fmt.Sprintf("\nThe previous key (%s) is no longer used by any account.\nDelete it?", result.OldKeyPath), false)
		if deleteOld {
			if err := deps.SSH.DeleteKey(result.OldKeyPath); err != nil {
				fmt.Fprintf(deps.Stderr, "git-rodolfo: %v\n", err)
				return 1
			}
			fmt.Fprintln(deps.Stdout, "✓ Previous key files deleted.")
		}
	}
	return 0
}

// printAccountEditDiff renders the changed fields (§13.3's before/after
// block) and, when applicable, the repositories-affected warning.
func printAccountEditDiff(deps Deps, result app.AccountEditResult) {
	before, after := result.Before, result.After
	printFieldDiff := func(label, oldVal, newVal string) {
		if oldVal == newVal {
			return
		}
		fmt.Fprintf(deps.Stdout, "\n%s:\n%s\n\u2192 %s\n", label, oldVal, newVal)
	}

	printFieldDiff("Display name", before.DisplayName, after.DisplayName)
	printFieldDiff("Commit name", before.GitName, after.GitName)
	printFieldDiff("Commit email", before.GitEmail, after.GitEmail)
	printFieldDiff("Provider username", before.ProviderUsername, after.ProviderUsername)
	printFieldDiff("SSH key", before.PrivateKeyPath, after.PrivateKeyPath)

	if result.RepoConfigAffected {
		fmt.Fprintln(deps.Stdout, "\nThis account is used by repositories configured with Git Rodolfo.")
		fmt.Fprintf(deps.Stdout, "\nRun \"git rodolfo use %s\" inside each affected repository\n", after.ID)
		fmt.Fprintln(deps.Stdout, "to apply the updated configuration.")
	}
}
