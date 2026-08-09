// Command git-rodolfo is the entrypoint described in PRD §12.1: it runs
// identically whether invoked directly as "git-rodolfo <command>" or, via
// Git's external-subcommand mechanism, as "git rodolfo <command>" — Git
// strips "rodolfo" and execs this binary with the remaining arguments
// unchanged, so no special-casing is needed here (CU-12).
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/lean-tech/git-rodolfo/internal/app"
	"github.com/lean-tech/git-rodolfo/internal/cli"
	"github.com/lean-tech/git-rodolfo/internal/infra"
)

// defaultReleaseRepo is where "git-rodolfo update" looks for releases,
// matching scripts/install.sh's own default. Override with
// GIT_RODOLFO_REPO (e.g. for a fork), same as that script.
const defaultReleaseRepo = "cristhian2121/git-rodolfo"

func main() {
	path, err := infra.DefaultConfigPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "git-rodolfo: %v\n", err)
		os.Exit(1)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "git-rodolfo: %v\n", err)
		os.Exit(1)
	}

	repo := infra.NewConfigRepository(path)
	sshClient := infra.NewSSHClient()
	agent := infra.NewSSHAgentAdapter()
	provider := infra.NewGitHubCLIAdapter()
	mechanism := app.NewSSHCommandMechanism()
	newGitClient := func(dir string) app.GitClient { return infra.NewGitClient(dir) }

	releaseRepo := os.Getenv("GIT_RODOLFO_REPO")
	if releaseRepo == "" {
		releaseRepo = defaultReleaseRepo
	}
	releaseFetcher := infra.NewGitHubReleaseFetcher(releaseRepo)
	assetDownloader := infra.NewHTTPAssetDownloader()

	deps := cli.Deps{
		Accounts:        app.NewAccountService(repo),
		AccountAdd:      app.NewAccountAddService(repo, sshClient, agent, time.Now),
		Edit:            app.NewAccountEditService(repo, sshClient, time.Now),
		Remove:          app.NewAccountRemoveService(repo, sshClient),
		Auth:            app.NewAuthenticationService(sshClient, repo, time.Now),
		Clone:           app.NewCloneService(infra.NewGitClient(""), newGitClient, mechanism),
		ErrorTranslator: app.NewErrorTranslator(repo),
		Update:          app.NewUpdateService(releaseFetcher, assetDownloader, runtime.GOOS, runtime.GOARCH, cli.Version),
		Env:             infra.NewEnvironmentInspector(),
		SSH:             sshClient,
		Agent:           agent,
		Provider:        provider,
		SelfUpdater:     infra.NewSelfUpdater(),
		AccountRepo:     repo,
		Mechanism:       mechanism,
		NewGitClient:    newGitClient,
		SSHDir:          filepath.Join(home, ".ssh"),
		Stdout:          os.Stdout,
		Stderr:          os.Stderr,
		Stdin:           os.Stdin,
		Now:             time.Now,
	}

	os.Exit(cli.Run(os.Args[1:], deps))
}
