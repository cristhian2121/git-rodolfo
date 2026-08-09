package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/lean-tech/git-rodolfo/internal/app"
	"github.com/lean-tech/git-rodolfo/internal/cli/prompt"
	"github.com/lean-tech/git-rodolfo/internal/domain"
)

// runClone implements "git rodolfo clone <url> [--account <id>]
// [--keep-https]" (RF-15, RF-16, RF-17, RF-18, RF-25, §13.6). The URL is
// expected first, as documented — it's pulled off before handing the rest
// to flag.FlagSet, since Go's flag package stops parsing flags at the
// first non-flag argument it sees (it doesn't permute like getopt), which
// would otherwise treat --account/--keep-https as further positional
// arguments once the URL comes first.
func runClone(deps Deps, flags GlobalFlags, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(deps.Stderr, "git-rodolfo: \"clone\" requires a repository URL")
		return 1
	}
	rawURL := args[0]

	var accountID string
	var keepHTTPS bool
	fs := flag.NewFlagSet("clone", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&accountID, "account", "", "account to clone with")
	fs.BoolVar(&keepHTTPS, "keep-https", false, "keep the HTTPS URL instead of switching to SSH")
	if err := fs.Parse(args[1:]); err != nil {
		fmt.Fprintf(deps.Stderr, "git-rodolfo: %v\n", err)
		return 1
	}
	if extra := fs.Args(); len(extra) > 0 {
		fmt.Fprintf(deps.Stderr, "git-rodolfo: \"clone\" takes a single repository URL (unexpected argument %q)\n", extra[0])
		return 1
	}

	u, err := domain.ParseRepositoryURL(rawURL)
	if err != nil {
		fmt.Fprintf(deps.Stderr, "git-rodolfo: %v\n", err)
		return 1
	}
	if err := app.ValidateHost(u, rawURL); err != nil {
		printCloneError(deps, err)
		return 1
	}

	// One session for the whole command: a second bufio-buffered Session
	// on the same Stdin can silently swallow input meant for a later
	// prompt (this bites when both the HTTPS->SSH choice and the account
	// selector need to run in the same invocation).
	session := prompt.NewSession(deps.Stdout, deps.Stdin)

	forceSSH := true
	if u.Scheme == domain.RepositoryURLSchemeHTTPS {
		switch {
		case keepHTTPS:
			forceSSH = false
		case flags.NonInteractive:
			fmt.Fprintln(deps.Stderr, "git-rodolfo: this is an HTTPS URL; pass --keep-https or use the SSH form of the URL in --non-interactive mode")
			return 1
		default:
			var code int
			forceSSH, u, code = resolveHTTPSChoice(deps, session, u)
			if code != 0 {
				return code
			}
		}
	}

	var account domain.Account
	if accountID != "" {
		account, err = deps.Accounts.Get(accountID)
		if err != nil {
			printAccountKeyError(deps, err)
			return 1
		}
	} else if flags.NonInteractive {
		fmt.Fprintln(deps.Stderr, "git-rodolfo: \"clone\" requires --account in --non-interactive mode")
		return 1
	} else {
		chosen, code := selectAccountInteractively(deps, session, "Select an account:")
		if code != 0 {
			return code
		}
		account, err = deps.Accounts.Get(chosen)
		if err != nil {
			printAccountKeyError(deps, err)
			return 1
		}
	}

	destination := u.Repo
	if err := deps.Clone.Clone(u, destination, account, forceSSH); err != nil {
		if deps.ErrorTranslator != nil {
			if msg, translated := deps.ErrorTranslator.Translate(err, app.OperationContext{Account: account, RemoteURL: u.String()}); translated {
				fmt.Fprintln(deps.Stderr, msg)
				return 1
			}
		}
		fmt.Fprintf(deps.Stderr, "git-rodolfo: %v\n", err)
		return 1
	}

	fmt.Fprintln(deps.Stdout, "Repository cloned successfully.")
	fmt.Fprintln(deps.Stdout)
	fmt.Fprintf(deps.Stdout, "Repository: %s\n", destination)
	fmt.Fprintf(deps.Stdout, "Account: %s\n", account.DisplayName)
	fmt.Fprintf(deps.Stdout, "Commit name: %s\n", account.GitName)
	fmt.Fprintf(deps.Stdout, "Commit email: %s\n", account.GitEmail)
	if forceSSH && account.PrivateKeyPath != "" {
		fmt.Fprintf(deps.Stdout, "SSH key: %s\n", account.PrivateKeyPath)
	}
	fmt.Fprintf(deps.Stdout, "Remote: %s\n", u.String())
	if !forceSSH {
		fmt.Fprintln(deps.Stdout, "\nAuthentication uses your HTTPS credential helper, not Git Rodolfo's SSH key.")
	}
	return 0
}

// resolveHTTPSChoice asks RF-16's required question — never converting
// HTTPS to SSH silently — and returns the resolved forceSSH flag and
// (possibly rewritten to SSH) URL.
func resolveHTTPSChoice(deps Deps, session *prompt.Session, u domain.RepositoryURL) (forceSSH bool, resolved domain.RepositoryURL, exitCode int) {
	sshURL := domain.RepositoryURL{Scheme: domain.RepositoryURLSchemeSSH, Host: u.Host, Owner: u.Owner, Repo: u.Repo}
	choice, err := session.Select("This is an HTTPS URL. Git Rodolfo authenticates with SSH.", []string{
		fmt.Sprintf("Clone using SSH (%s)", sshURL.String()),
		"Keep HTTPS — Git Rodolfo will only configure name and email",
		"Cancel",
	})
	if errors.Is(err, prompt.ErrCanceled) || (err == nil && choice == 2) {
		fmt.Fprintln(deps.Stderr, "Canceled.")
		return false, u, 1
	}
	if err != nil {
		fmt.Fprintf(deps.Stderr, "git-rodolfo: %v\n", err)
		return false, u, 1
	}
	if choice == 0 {
		return true, sshURL, 0
	}
	return false, u, 0
}

func printCloneError(deps Deps, err error) {
	var unsupported *app.ErrUnsupportedHost
	if errors.As(err, &unsupported) {
		fmt.Fprintln(deps.Stderr, "Git Rodolfo only supports GitHub in this version.")
		fmt.Fprintln(deps.Stderr)
		fmt.Fprintln(deps.Stderr, "Remote:")
		fmt.Fprintln(deps.Stderr, unsupported.Remote)
		fmt.Fprintln(deps.Stderr)
		fmt.Fprintln(deps.Stderr, "No changes were made.")
		return
	}
	fmt.Fprintf(deps.Stderr, "git-rodolfo: %v\n", err)
}
