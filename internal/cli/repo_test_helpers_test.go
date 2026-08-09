package cli_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/lean-tech/git-rodolfo/internal/app"
	"github.com/lean-tech/git-rodolfo/internal/cli"
	"github.com/lean-tech/git-rodolfo/internal/infra/fakes"
)

// repoDeps builds a Deps for "use"/"current"/"clone" tests: a fixed
// working directory bound to a single FakeGitClient (NewGitClient ignores
// its dir argument and always returns git, matching how these commands
// only ever touch one repository per invocation in tests).
func repoDeps(t *testing.T, stdin *os.File, repo *fakes.FakeAccountRepository, git *fakes.FakeGitClient) (cli.Deps, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	mechanism := app.NewSSHCommandMechanism()
	return cli.Deps{
		Accounts:        app.NewAccountService(repo),
		AccountRepo:     repo,
		Mechanism:       mechanism,
		NewGitClient:    func(dir string) app.GitClient { return git },
		Clone:           app.NewCloneService(git, func(dir string) app.GitClient { return git }, mechanism),
		ErrorTranslator: app.NewErrorTranslator(repo),
		SSH:             fakes.NewFakeSSHClient(),
		Env:             fakes.NewFakeEnvironmentInspector(),
		Getwd:           func() (string, error) { return "/repo/project", nil },
		Stdout:          &stdout,
		Stderr:          &stderr,
		Stdin:           stdin,
		Now:             fixedNow,
	}, &stdout, &stderr
}
