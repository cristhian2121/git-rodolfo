# Git Rodolfo

Manage multiple Git/GitHub identities on one computer. See `git -rodolfo-PRD.md`
for the product spec and `git -rodolfo-SPRINTS.md` for the delivery plan.

**Status:** MVP complete (Sprints 1–6). All commands in the PRD are
implemented: `accounts`, `account show/add/edit/remove`, `clone`, `use`,
`current`, `doctor`.

## Install

Build from source (no published release yet — see `Formula/git-rodolfo.rb`
and `scripts/install.sh` for what a release-based install looks like once
one exists):

```bash
make install   # installs to $(go env GOPATH)/bin as git-rodolfo
```

Make sure that directory is on `PATH` — Git looks up `git-rodolfo` there
to make `git rodolfo <command>` work (PRD §12.1).

## Build

```bash
make build      # ./git-rodolfo, version embedded from `git describe`
make test       # go vet + go test ./... -race
make release    # cross-compiled tarballs for darwin/linux × amd64/arm64 in dist/
```

## Try it locally

```bash
./git-rodolfo account add          # interactive wizard
./git-rodolfo accounts
git rodolfo accounts                # same binary, via Git's subcommand mechanism
```

Accounts are stored in `~/.config/git-rodolfo/config.json` (or
`$XDG_CONFIG_HOME/git-rodolfo/config.json`). `examples/config.example.json`
is a fixture you can copy there to explore the read-only commands (`accounts`,
`account show`) without registering a real account first.

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
