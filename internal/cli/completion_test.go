package cli_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/lean-tech/git-rodolfo/internal/cli"
)

func TestRun_Completion_KnownShells(t *testing.T) {
	tests := []struct {
		shell string
		want  string
	}{
		{"bash", "complete -F _git_rodolfo_completions git-rodolfo"},
		{"zsh", "#compdef git-rodolfo"},
		{"fish", "complete -c git-rodolfo"},
	}
	for _, tt := range tests {
		t.Run(tt.shell, func(t *testing.T) {
			deps, stdout, stderr := newDeps()
			code := cli.Run([]string{"completion", tt.shell}, deps)
			if code != 0 {
				t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
			}
			if !strings.Contains(stdout.String(), tt.want) {
				t.Fatalf("output missing %q:\n%s", tt.want, stdout.String())
			}
		})
	}
}

func TestRun_Completion_MissingShell(t *testing.T) {
	deps, _, stderr := newDeps()
	code := cli.Run([]string{"completion"}, deps)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "requires a shell") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestRun_Completion_UnsupportedShell(t *testing.T) {
	deps, _, stderr := newDeps()
	code := cli.Run([]string{"completion", "powershell"}, deps)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), `unsupported shell "powershell"`) {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

// TestBashCompletionScript_IsSyntacticallyValid catches typos in the
// hand-written bash script via `bash -n` (parse-only, doesn't execute
// it), if bash is available.
func TestBashCompletionScript_IsSyntacticallyValid(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not found on PATH")
	}
	deps, stdout, stderr := newDeps()
	if code := cli.Run([]string{"completion", "bash"}, deps); code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}

	f, err := os.CreateTemp(t.TempDir(), "git-rodolfo-completion-*.bash")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	if _, err := f.WriteString(stdout.String()); err != nil {
		t.Fatalf("WriteString: %v", err)
	}
	f.Close()

	out, err := exec.Command("bash", "-n", f.Name()).CombinedOutput()
	if err != nil {
		t.Fatalf("bash -n reported a syntax error: %v\n%s", err, out)
	}
}

// TestZshCompletionScript_IsSyntacticallyValid mirrors the bash check
// above, using `zsh -n`.
func TestZshCompletionScript_IsSyntacticallyValid(t *testing.T) {
	if _, err := exec.LookPath("zsh"); err != nil {
		t.Skip("zsh not found on PATH")
	}
	deps, stdout, stderr := newDeps()
	if code := cli.Run([]string{"completion", "zsh"}, deps); code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}

	f, err := os.CreateTemp(t.TempDir(), "git-rodolfo-completion-*.zsh")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	if _, err := f.WriteString(stdout.String()); err != nil {
		t.Fatalf("WriteString: %v", err)
	}
	f.Close()

	out, err := exec.Command("zsh", "-n", f.Name()).CombinedOutput()
	if err != nil {
		t.Fatalf("zsh -n reported a syntax error: %v\n%s", err, out)
	}
}

// TestFishCompletionScript_IsSyntacticallyValid uses `fish --no-execute`,
// fish's parse-only mode, the same idea as bash -n / zsh -n.
func TestFishCompletionScript_IsSyntacticallyValid(t *testing.T) {
	if _, err := exec.LookPath("fish"); err != nil {
		t.Skip("fish not found on PATH")
	}
	deps, stdout, stderr := newDeps()
	if code := cli.Run([]string{"completion", "fish"}, deps); code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}

	f, err := os.CreateTemp(t.TempDir(), "git-rodolfo-completion-*.fish")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	if _, err := f.WriteString(stdout.String()); err != nil {
		t.Fatalf("WriteString: %v", err)
	}
	f.Close()

	out, err := exec.Command("fish", "--no-execute", f.Name()).CombinedOutput()
	if err != nil {
		t.Fatalf("fish --no-execute reported a syntax error: %v\n%s", err, out)
	}
}
