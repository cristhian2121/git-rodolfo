package cli

import (
	"fmt"

	"github.com/lean-tech/git-rodolfo/internal/app"
)

// runDoctor implements "git rodolfo doctor" (RF-22, §13.10): reports and
// suggests, never fixes anything itself. It exits non-zero when issues are
// found, so it can gate scripts/CI the way `brew doctor` does.
//
// Validations #6/#7 (current repository consistency, remote host) only
// apply when doctor runs inside a Git repository — that's determined here,
// at invocation time, rather than baked into Deps, since it depends on the
// working directory the command happens to run in.
func runDoctor(deps Deps, args []string) int {
	if len(args) > 0 {
		fmt.Fprintf(deps.Stderr, "git-rodolfo: \"doctor\" takes no arguments (got %q)\n", args[0])
		return 1
	}

	var git app.GitClient
	if dir, err := deps.Getwd(); err == nil {
		candidate := deps.NewGitClient(dir)
		if candidate.IsRepository(dir) {
			git = candidate
		}
	}

	diagnostic := app.NewDiagnosticService(deps.AccountRepo, deps.SSH, deps.Env, git, deps.Mechanism)
	findings, err := diagnostic.Run()
	if err != nil {
		fmt.Fprintf(deps.Stderr, "git-rodolfo: %v\n", err)
		return 1
	}

	fmt.Fprintln(deps.Stdout, "Git Rodolfo Doctor")
	fmt.Fprintln(deps.Stdout)

	failedCount := 0
	for _, f := range findings {
		mark := "✓"
		if !f.OK {
			mark = "✗"
			failedCount++
		}
		fmt.Fprintf(deps.Stdout, "%s %s\n", mark, f.Message)
	}

	fmt.Fprintln(deps.Stdout)
	if failedCount == 0 {
		fmt.Fprintln(deps.Stdout, "No issues found.")
		return 0
	}
	if failedCount == 1 {
		fmt.Fprintln(deps.Stdout, "1 issue found.")
	} else {
		fmt.Fprintf(deps.Stdout, "%d issues found.\n", failedCount)
	}

	for _, f := range findings {
		if f.OK {
			continue
		}
		fmt.Fprintln(deps.Stdout)
		fmt.Fprintln(deps.Stdout, f.Message)
		if len(f.Causes) > 0 {
			fmt.Fprintln(deps.Stdout, "Possible causes:")
			for _, c := range f.Causes {
				fmt.Fprintf(deps.Stdout, "  - %s\n", c)
			}
		}
		if f.Command != "" {
			fmt.Fprintf(deps.Stdout, "Run: %s\n", f.Command)
		}
	}
	return 1
}
