package cli_test

import (
	"strings"
	"testing"

	"github.com/lean-tech/git-rodolfo/internal/cli"
)

func TestRun_PerCommandHelp(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"accounts", "--help"}, "Usage: git-rodolfo accounts"},
		{[]string{"account", "--help"}, "Usage: git-rodolfo account <show|add|edit|remove>"},
		{[]string{"account", "-h"}, "Usage: git-rodolfo account <show|add|edit|remove>"},
		{[]string{"account", "show", "--help"}, "Usage: git-rodolfo account show <account>"},
		{[]string{"account", "add", "--help"}, "Usage: git-rodolfo account add [flags]"},
		{[]string{"account", "edit", "--help"}, "Usage: git-rodolfo account edit [account] [flags]"},
		{[]string{"account", "remove", "--help"}, "Usage: git-rodolfo account remove [account] [--delete-key]"},
		{[]string{"clone", "--help"}, "Usage: git-rodolfo clone <repository-url>"},
		{[]string{"use", "--help"}, "Usage: git-rodolfo use [account] [--clear]"},
		{[]string{"current", "--help"}, "Usage: git-rodolfo current"},
		{[]string{"doctor", "--help"}, "Usage: git-rodolfo doctor"},
		{[]string{"completion", "--help"}, "Usage: git-rodolfo completion <bash|zsh|fish>"},
		// A subcommand's own flags/args must not prevent --help from
		// short-circuiting, regardless of where it appears.
		{[]string{"account", "show", "lean-tech", "--help"}, "Usage: git-rodolfo account show <account>"},
		{[]string{"clone", "git@github.com:x/y.git", "--account", "x", "--help"}, "Usage: git-rodolfo clone <repository-url>"},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			deps, stdout, stderr := newDeps()
			code := cli.Run(tt.args, deps)
			if code != 0 {
				t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
			}
			if !strings.Contains(stdout.String(), tt.want) {
				t.Fatalf("output missing %q:\n%s", tt.want, stdout.String())
			}
		})
	}
}

// TestRun_PerCommandHelp_DoesNotFireForUnknownCommand confirms an unknown
// top-level command with --help still falls through to the normal
// "unknown command" error instead of silently doing nothing or crashing.
func TestRun_PerCommandHelp_DoesNotFireForUnknownCommand(t *testing.T) {
	deps, _, stderr := newDeps()
	code := cli.Run([]string{"bogus", "--help"}, deps)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), `unknown command "bogus"`) {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}
