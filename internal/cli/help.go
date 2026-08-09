package cli

import (
	"fmt"
	"io"
)

// Version is the tool's version string. The release build overrides it via
// -ldflags "-X .../internal/cli.Version=..." (see Makefile); it stays
// "0.1.0-dev" for `go build`/`go run` without that flag.
var Version = "0.1.0-dev"

const helpText = `git-rodolfo — manage multiple Git/GitHub identities on one computer

Usage:
  git-rodolfo <command> [arguments]
  git rodolfo <command> [arguments]

Commands:
  accounts                       List registered accounts
  account show <account>         Show details for one account
  account add                    Register a new account
  account edit [account]         Edit a registered account
  account remove [account]       Remove a registered account
  clone <repository-url>         Clone a repository with a chosen account
  use [account] [--clear]        Assign (or clear) an account for this repository
  current                        Show which account this repository is using
  doctor                         Diagnose Git/SSH/GitHub configuration issues
  completion <bash|zsh|fish>     Print a shell completion script
  update [--version <tag>]       Update git-rodolfo itself to the latest (or a given) release
  help                           Show this help text

Run "<command> --help" (e.g. "account add --help") for details on a
specific command.

Global flags:
  --non-interactive            Fail instead of prompting
  --yes                        Assume "yes" for non-destructive confirmations
  --verbose                    Print the underlying commands being run

Run "git-rodolfo --version" (or "-v") to print the version.
`

func printHelp(w io.Writer) {
	fmt.Fprint(w, helpText)
}

func printVersion(w io.Writer) {
	fmt.Fprintf(w, "git-rodolfo version %s\n", Version)
}
