// Package cli is the CLI layer (PRD §19.2): argument parsing and output
// formatting only. It holds no business logic — that lives in internal/app.
package cli

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/lean-tech/git-rodolfo/internal/app"
)

// Deps are the dependencies the CLI layer needs, injected so tests can run
// against fakes instead of a real config file, clock or terminal.
type Deps struct {
	Accounts        *app.AccountService
	AccountAdd      *app.AccountAddService
	Edit            *app.AccountEditService
	Remove          *app.AccountRemoveService
	Auth            *app.AuthenticationService
	Clone           *app.CloneService
	ErrorTranslator *app.ErrorTranslator
	Update          *app.UpdateService
	Env             app.EnvironmentInspector
	SSH             app.SSHClient
	Agent           app.SSHAgentAdapter
	Provider        app.ProviderClient
	SelfUpdater     app.SelfUpdater

	// AccountRepo, Mechanism and NewGitClient build a
	// RepositoryIdentityService bound to the current working directory —
	// "use" and "current" need a GitClient scoped to wherever the command
	// is run, which isn't known until then, so Deps carries the pieces
	// instead of an already-built service.
	AccountRepo  app.AccountRepository
	Mechanism    app.IdentityMechanism
	NewGitClient func(dir string) app.GitClient
	// Getwd resolves the working directory "use"/"current" operate on;
	// defaults to os.Getwd if nil.
	Getwd func() (string, error)

	// SSHDir is where new keys are suggested by default (normally
	// ~/.ssh), and Stdin is used both for line-based prompts and, when it
	// is a real terminal, for the arrow-key account/option selectors.
	SSHDir string
	Stdout io.Writer
	Stderr io.Writer
	Stdin  *os.File
	Now    func() time.Time
}

// Run parses args (excluding the program name) and dispatches to a
// subcommand. It returns the process exit code — 0 on success, 1 on any
// user-facing error — so main() can call os.Exit directly.
func Run(args []string, deps Deps) int {
	if deps.Now == nil {
		deps.Now = time.Now
	}
	if deps.Getwd == nil {
		deps.Getwd = os.Getwd
	}

	globalFlags, rest := extractGlobalFlags(args)

	if len(rest) == 0 {
		printHelp(deps.Stdout)
		return 0
	}

	// "<command> --help" / "<command> -h" short-circuits before any
	// subcommand parses its own flags, so it never depends on (or
	// fights with) each command's own flag.FlagSet or manual parsing.
	if hasHelpFlag(rest[1:]) {
		if text, ok := commandHelpText[commandHelpKey(rest)]; ok {
			fmt.Fprint(deps.Stdout, text)
			return 0
		}
	}

	switch rest[0] {
	case "help", "--help", "-h":
		printHelp(deps.Stdout)
		return 0
	case "--version", "-v":
		printVersion(deps.Stdout)
		return 0
	case "accounts":
		return runAccounts(deps, rest[1:])
	case "account":
		return runAccount(deps, globalFlags, rest[1:])
	case "clone":
		return runClone(deps, globalFlags, rest[1:])
	case "use":
		return runUse(deps, globalFlags, rest[1:])
	case "current":
		return runCurrent(deps, rest[1:])
	case "doctor":
		return runDoctor(deps, rest[1:])
	case "completion":
		return runCompletion(deps, rest[1:])
	case "update":
		return runUpdate(deps, globalFlags, rest[1:])
	default:
		fmt.Fprintf(deps.Stderr, "git-rodolfo: unknown command %q\n\n", rest[0])
		printHelp(deps.Stderr)
		return 1
	}
}
