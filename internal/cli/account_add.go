package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/lean-tech/git-rodolfo/internal/app"
	"github.com/lean-tech/git-rodolfo/internal/cli/prompt"
	"github.com/lean-tech/git-rodolfo/internal/domain"
)

// runAccountAdd implements "git rodolfo account add" (RF-01, §13.2). Flags
// (and --non-interactive) go through the flag-driven path; otherwise it
// runs the interactive wizard.
func runAccountAdd(deps Deps, flags GlobalFlags, args []string) int {
	if flags.NonInteractive || len(args) > 0 {
		return runAccountAddNonInteractive(deps, flags, args)
	}
	return runAccountAddInteractive(deps)
}

type accountAddFlags struct {
	name           string
	username       string
	commitName     string
	commitEmail    string
	key            string
	generateKey    bool
	keyPath        string
	overwrite      bool
	configureLater bool
	registerWithGH bool
}

func runAccountAddNonInteractive(deps Deps, flags GlobalFlags, args []string) int {
	if !flags.NonInteractive {
		fmt.Fprintln(deps.Stderr, "git-rodolfo: \"account add\" flags require --non-interactive")
		return 1
	}

	var f accountAddFlags
	fs := flag.NewFlagSet("account add", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&f.name, "name", "", "account display name")
	fs.StringVar(&f.username, "username", "", "provider (GitHub) username")
	fs.StringVar(&f.commitName, "commit-name", "", "git commit name")
	fs.StringVar(&f.commitEmail, "commit-email", "", "git commit email")
	fs.StringVar(&f.key, "key", "", "path to an existing private key to associate")
	fs.BoolVar(&f.generateKey, "generate-key", false, "generate a new Ed25519 key")
	fs.StringVar(&f.keyPath, "key-path", "", "target path when --generate-key is set (default: ~/.ssh/id_ed25519_<account-id>)")
	fs.BoolVar(&f.overwrite, "overwrite", false, "allow --generate-key to overwrite an existing file at --key-path")
	fs.BoolVar(&f.configureLater, "configure-later", false, "register the account without an SSH key yet")
	fs.BoolVar(&f.registerWithGH, "register-key-with-gh", false, "register the public key on GitHub via gh (requires --key or --generate-key)")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(deps.Stderr, "git-rodolfo: %v\n", err)
		return 1
	}
	if extra := fs.Args(); len(extra) > 0 {
		fmt.Fprintf(deps.Stderr, "git-rodolfo: \"account add\" does not take positional arguments (got %q)\n", extra[0])
		return 1
	}

	if f.name == "" || f.username == "" || f.commitName == "" || f.commitEmail == "" {
		fmt.Fprintln(deps.Stderr, "git-rodolfo: --name, --username, --commit-name and --commit-email are all required in --non-interactive mode")
		return 1
	}

	chosen := 0
	if f.key != "" {
		chosen++
	}
	if f.generateKey {
		chosen++
	}
	if f.configureLater {
		chosen++
	}
	if chosen != 1 {
		fmt.Fprintln(deps.Stderr, "git-rodolfo: specify exactly one of --key, --generate-key or --configure-later")
		return 1
	}
	if f.registerWithGH && f.configureLater {
		fmt.Fprintln(deps.Stderr, "git-rodolfo: --register-key-with-gh requires --key or --generate-key")
		return 1
	}

	input := app.AccountAddInput{
		DisplayName:      f.name,
		ProviderUsername: f.username,
		GitName:          f.commitName,
		GitEmail:         f.commitEmail,
	}

	switch {
	case f.key != "":
		input.KeyMode = app.KeyModeExisting
		input.KeyPath = expandHome(f.key)
	case f.generateKey:
		input.KeyMode = app.KeyModeGenerate
		input.OverwriteKey = f.overwrite
		if f.keyPath != "" {
			input.KeyPath = expandHome(f.keyPath)
		} else {
			input.KeyPath = filepath.Join(deps.SSHDir, "id_ed25519_"+app.Slugify(f.name))
		}
		// A passphrase prompt has nowhere to go in --non-interactive mode
		// (RF-11 forbids Git Rodolfo from ever supplying one itself), so
		// generated keys are always created without one here.
		input.WithPassphrase = false
	case f.configureLater:
		input.KeyMode = app.KeyModeConfigureLater
	}

	account, err := deps.AccountAdd.Add(input)
	if err != nil {
		printAccountKeyError(deps, err)
		return 1
	}

	registration := ""
	if f.registerWithGH {
		registration = "gh"
	}
	return finishAccountAdd(deps, account, input.KeyMode, registration)
}

func runAccountAddInteractive(deps Deps) int {
	session := prompt.NewSession(deps.Stdout, deps.Stdin)

	name, err := session.Text("Account name")
	if err != nil || name == "" {
		fmt.Fprintln(deps.Stderr, "git-rodolfo: account name is required")
		return 1
	}
	username, _ := session.Text("Provider username")
	commitName, _ := session.Text("Commit name")
	commitEmail, _ := session.Text("Commit email")

	input := app.AccountAddInput{
		DisplayName:      name,
		ProviderUsername: username,
		GitName:          commitName,
		GitEmail:         commitEmail,
	}

	keyChoice, err := session.Select("SSH key", []string{
		"Generate a new key",
		"Use an existing key",
		"Configure later",
	})
	if errors.Is(err, prompt.ErrCanceled) {
		fmt.Fprintln(deps.Stderr, "Canceled.")
		return 1
	}
	if err != nil {
		fmt.Fprintf(deps.Stderr, "git-rodolfo: %v\n", err)
		return 1
	}

	switch keyChoice {
	case 0:
		if code := fillGenerateKey(session, deps, &input); code != 0 {
			return code
		}
	case 1:
		if code := fillExistingKey(session, deps, &input); code != 0 {
			return code
		}
	case 2:
		input.KeyMode = app.KeyModeConfigureLater
	}

	registration := ""
	if input.KeyMode != app.KeyModeConfigureLater {
		regChoice, err := session.Select("How do you want to register the public key?", []string{
			"Add using GitHub CLI",
			"Copy the public key to the clipboard",
			"Configure later",
		})
		if err == nil {
			switch regChoice {
			case 0:
				registration = "gh"
			case 1:
				registration = "clipboard"
			}
		}
	}

	account, err := deps.AccountAdd.Add(input)
	if err != nil {
		printAccountKeyError(deps, err)
		return 1
	}

	return finishAccountAdd(deps, account, input.KeyMode, registration)
}

func fillGenerateKey(session *prompt.Session, deps Deps, input *app.AccountAddInput) int {
	defaultFilename := "id_ed25519_" + app.Slugify(input.DisplayName)
	filename, _ := session.Text(fmt.Sprintf("SSH key filename (default: %s)", defaultFilename))
	if filename == "" {
		filename = defaultFilename
	}
	path := filepath.Join(deps.SSHDir, filename)

	withPassphrase, _ := session.Confirm("Add a passphrase?", false)

	overwrite := false
	if _, err := os.Stat(path); err == nil {
		// RF-06: never overwrite an existing file without explicit
		// confirmation, whether or not it's actually a valid key.
		overwrite, _ = session.Confirm(fmt.Sprintf("%s already exists. Overwrite?", path), false)
		if !overwrite {
			fmt.Fprintln(deps.Stderr, "Aborted: key file already exists.")
			return 1
		}
	}

	input.KeyMode = app.KeyModeGenerate
	input.KeyPath = path
	input.WithPassphrase = withPassphrase
	input.OverwriteKey = overwrite

	if withPassphrase {
		fillAgentChoice(session, deps, input)
	}
	return 0
}

func fillExistingKey(session *prompt.Session, deps Deps, input *app.AccountAddInput) int {
	raw, _ := session.Text("Path to the private key")
	path := expandHome(raw)

	hasPassphrase, err := deps.SSH.KeyHasPassphrase(path)
	if err != nil {
		fmt.Fprintf(deps.Stderr, "git-rodolfo: %v\n", err)
		return 1
	}

	input.KeyMode = app.KeyModeExisting
	input.KeyPath = path

	if hasPassphrase {
		fillAgentChoice(session, deps, input)
	}
	return 0
}

func fillAgentChoice(session *prompt.Session, deps Deps, input *app.AccountAddInput) {
	options := []string{
		"Add it to ssh-agent now (recommended)",
	}
	if runtime.GOOS == "darwin" {
		options = append(options, "Add it to ssh-agent and remember it in the macOS Keychain")
	}
	options = append(options, "Skip — I'll enter the passphrase each time")

	fmt.Fprintln(deps.Stdout, "\nThis key is protected with a passphrase.")
	choice, err := session.Select("", options)
	if err != nil {
		return // treat any prompt failure as "skip"; nothing was loaded
	}

	switch options[choice] {
	case "Add it to ssh-agent now (recommended)":
		input.LoadIntoAgent = true
	case "Add it to ssh-agent and remember it in the macOS Keychain":
		input.LoadIntoAgent = true
		input.UseKeychain = true
	}
}

// finishAccountAdd handles what happens once the account is already saved:
// optionally registering the public key on GitHub (RF-14), then running
// the RF-13 authentication test and reporting its result — the same
// closing sequence PRD §13.2 describes ("Testing authentication..." /
// "✓ SSH key accepted" / "✓ GitHub account detected" / "✓ Account saved").
// Registration or verification trouble is reported with a non-zero exit,
// but never undoes the save: the account already exists either way.
func finishAccountAdd(deps Deps, account domain.Account, keyMode app.KeyMode, registration string) int {
	exit := 0

	if keyMode != app.KeyModeConfigureLater {
		if registration != "" {
			if code := registerPublicKey(deps, account, registration); code != 0 {
				exit = code
			}
		}

		fmt.Fprintln(deps.Stdout, "\nTesting authentication...")
		outcome, err := deps.Auth.Verify(account)
		if err != nil {
			fmt.Fprintf(deps.Stderr, "git-rodolfo: %v\n", err)
			exit = 1
		} else {
			printVerifyOutcome(deps, outcome)
		}
	}

	fmt.Fprintf(deps.Stdout, "\n✓ Account %q saved.\n", account.DisplayName)
	return exit
}

func registerPublicKey(deps Deps, account domain.Account, mode string) int {
	pubKey, err := deps.SSH.PublicKeyContent(account.PrivateKeyPath)
	if err != nil {
		fmt.Fprintf(deps.Stderr, "git-rodolfo: could not read public key for %s: %v\n", account.PrivateKeyPath, err)
		return 1
	}

	switch mode {
	case "gh":
		if err := app.RegisterPublicKeyIfMatchingSession(deps.Provider, account, pubKey); err != nil {
			printGitHubCLIError(deps, err)
			return 1
		}
		fmt.Fprintln(deps.Stdout, "✓ Public key registered on GitHub.")
	case "clipboard":
		if err := copyToClipboard(pubKey); err != nil {
			fmt.Fprintln(deps.Stdout, "Could not access the clipboard. Copy this public key manually:")
			fmt.Fprintln(deps.Stdout, strings.TrimSpace(pubKey))
		} else {
			fmt.Fprintln(deps.Stdout, "✓ Public key copied to the clipboard.")
		}
	}
	return 0
}

// printVerifyOutcome renders RF-13's result, including §21's "the key
// authenticates as another user" case.
func printVerifyOutcome(deps Deps, outcome app.VerifyOutcome) {
	if !outcome.Result.Success {
		fmt.Fprintln(deps.Stdout, "✗ SSH key rejected by GitHub.")
		return
	}
	if outcome.UsernameMismatch {
		fmt.Fprintf(deps.Stdout, "\nThe key authenticated successfully, but as %q.\n\n", outcome.Result.Username)
		fmt.Fprintf(deps.Stdout, "You registered this account as %q.\n", outcome.Account.ProviderUsername)
		fmt.Fprintln(deps.Stdout, "This key belongs to a different GitHub account.")
		return
	}
	fmt.Fprintln(deps.Stdout, "✓ SSH key accepted")
	fmt.Fprintf(deps.Stdout, "✓ GitHub account detected: %s\n", outcome.Result.Username)
}

// printGitHubCLIError renders §21's "GitHub CLI is authenticated with
// another account" message for a GitHubCLIMismatchError, or a generic
// error line for anything else.
func printGitHubCLIError(deps Deps, err error) {
	var mismatch *app.GitHubCLIMismatchError
	if errors.As(err, &mismatch) {
		fmt.Fprintf(deps.Stderr, "GitHub CLI is authenticated as %q,\nbut this account is %q.\n\n", mismatch.ActiveUsername, mismatch.AccountUsername)
		fmt.Fprintln(deps.Stderr, "Adding the key now would register it on the wrong account.")
		fmt.Fprintln(deps.Stderr, "\nRun:\ngh auth switch")
		return
	}
	fmt.Fprintf(deps.Stderr, "git-rodolfo: %v\n", err)
}
