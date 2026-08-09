# Git Rodolfo

[![CI](https://github.com/cristhian2121/git-rodolfo/actions/workflows/ci.yml/badge.svg)](https://github.com/cristhian2121/git-rodolfo/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/cristhian2121/git-rodolfo)](https://github.com/cristhian2121/git-rodolfo/releases/latest)

Manage multiple Git/GitHub identities on one computer. See `git -rodolfo-PRD.md`
for the product spec and `git -rodolfo-SPRINTS.md` for the delivery plan.

**Status:** MVP complete (Sprints 1–6). All commands in the PRD are
implemented: `accounts`, `account show/add/edit/remove`, `clone`, `use`,
`current`, `doctor`, `update`, `completion`.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/cristhian2121/git-rodolfo/main/scripts/install.sh | bash
```

Downloads the right binary for your OS/arch from the
[latest release](https://github.com/cristhian2121/git-rodolfo/releases/latest)
and installs it to `/usr/local/bin` (override with `GIT_RODOLFO_INSTALL_DIR`).

Or build from source:

```bash
make install   # installs to $(go env GOPATH)/bin as git-rodolfo
```

Either way, make sure the install directory is on `PATH` — Git looks up
`git-rodolfo` there to make `git rodolfo <command>` work (PRD §12.1).

Already installed? Run `git-rodolfo update` to pull the latest release.

## Usage

```bash
git-rodolfo account add                       # register an account (interactive wizard)
git-rodolfo accounts                          # list registered accounts
git-rodolfo clone <url> --account <account>   # clone with the right SSH key
git-rodolfo use <account>                     # set the account for the current repo
git-rodolfo current                           # show which account a repo is using
git-rodolfo doctor                            # diagnose SSH/config issues
git-rodolfo update                            # self-update to the latest release
git-rodolfo completion bash|zsh|fish          # print a shell completion script
```

Run `git-rodolfo <command> --help` for full flags and examples on any
command, or `git-rodolfo help` for the complete list. `git-rodolfo <command>`
and `git rodolfo <command>` are the same binary — the latter works via Git's
subcommand mechanism once it's on `PATH`.

Accounts are stored in `~/.config/git-rodolfo/config.json` (or
`$XDG_CONFIG_HOME/git-rodolfo/config.json`). Copy `examples/config.example.json`
there to explore the read-only commands (`accounts`, `account show`) without
registering a real account first.

## Build

```bash
make build      # ./git-rodolfo, version embedded from `git describe`
make test       # go vet + go test ./... -race
make release    # cross-compiled tarballs for darwin/linux × amd64/arm64 in dist/
```

## Documentation

- `docs/TESTPLAN.md` — the manual end-to-end suite covering the PRD §22
  acceptance criteria that need two real GitHub accounts (duplicate-key
  rejection, the wrong-account clone scenario, etc.) — not automatable,
  so it's not part of `go test`.
- `docs/TROUBLESHOOTING.md` — fixes for the cases `doctor` reports but
  can't resolve itself, plus the automated suite's known coverage gaps.

## Layout

```
cmd/git-rodolfo       entrypoint
internal/domain       Account, GlobalConfig, ValidationResult, RepositoryURL — no dependencies
internal/app          Application layer: services + the ports in ports.go
internal/infra        Real adapters (Git, SSH, ssh-agent, GitHub CLI, config file) and fakes/
internal/cli          Argument parsing and output formatting — no business logic
internal/cli/prompt   Interactive prompts (arrow-key select, confirm, text)
```

## Test

```bash
go test ./... -race
```

Unit tests run against the fakes in `internal/infra/fakes` (RNF-09); several
files also contain guarded integration tests that exercise the real `git`,
`ssh-keygen` and `ssh-add` binaries against local repos/keys — no network,
no GitHub. They skip automatically if those tools aren't on `PATH`.
