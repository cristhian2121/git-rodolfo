package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/lean-tech/git-rodolfo/internal/app"
	"github.com/lean-tech/git-rodolfo/internal/cli/prompt"
)

// runUpdate implements "git rodolfo update [--version <tag>]" (§26):
// downloads the latest release (or a specific tag) and replaces the
// running binary, after explicit confirmation.
func runUpdate(deps Deps, flags GlobalFlags, args []string) int {
	var version string
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&version, "version", "", "update to this exact release tag instead of the latest (e.g. v0.2.0)")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(deps.Stderr, "git-rodolfo: %v\n", err)
		return 1
	}
	if extra := fs.Args(); len(extra) > 0 {
		fmt.Fprintf(deps.Stderr, "git-rodolfo: \"update\" does not take positional arguments (got %q)\n", extra[0])
		return 1
	}

	result, err := deps.Update.Update(version)
	if errors.Is(err, app.ErrAlreadyUpToDate) {
		fmt.Fprintf(deps.Stdout, "Already up to date (%s).\n", Version)
		return 0
	}
	if errors.Is(err, app.ErrReleaseAssetNotFound) {
		fmt.Fprintf(deps.Stderr, "git-rodolfo: %v\n", err)
		return 1
	}
	if err != nil {
		fmt.Fprintf(deps.Stderr, "git-rodolfo: %v\n", err)
		return 1
	}

	fmt.Fprintf(deps.Stdout, "%s -> %s available.\n", result.FromVersion, result.ToVersion)

	if !flags.Yes {
		if flags.NonInteractive {
			fmt.Fprintln(deps.Stderr, "git-rodolfo: \"update\" requires --yes in --non-interactive mode")
			return 1
		}
		session := prompt.NewSession(deps.Stdout, deps.Stdin)
		confirmed, _ := session.Confirm("Replace the current binary?", false)
		if !confirmed {
			fmt.Fprintln(deps.Stdout, "Aborted. No changes were made.")
			return 0
		}
	}

	if err := deps.SelfUpdater.Replace(result.BinaryData); err != nil {
		fmt.Fprintf(deps.Stderr, "git-rodolfo: %v\n", err)
		return 1
	}
	fmt.Fprintf(deps.Stdout, "✓ Updated to %s.\n", result.ToVersion)
	return 0
}
