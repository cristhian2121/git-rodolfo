package app

import (
	"errors"
	"fmt"

	"github.com/lean-tech/git-rodolfo/internal/domain"
)

// DiagnosticFinding is one line of "doctor" output (§13.10): OK findings
// need nothing more than Message; failing ones carry the causes and the
// exact command that fixes them (RF-22 — doctor reports, it never fixes).
type DiagnosticFinding struct {
	OK      bool
	Message string
	Causes  []string
	Command string
}

// DiagnosticService implements "git rodolfo doctor"'s eight validations
// (§13.10). Git/Mechanism may be nil when doctor isn't run inside a
// repository — validations 6-7 are then skipped, not failed, since they
// don't apply outside a repo.
type DiagnosticService struct {
	accounts  AccountRepository
	ssh       SSHClient
	env       EnvironmentInspector
	git       GitClient
	mechanism IdentityMechanism
}

// NewDiagnosticService builds a DiagnosticService. git and mechanism may
// be nil (see the type doc).
func NewDiagnosticService(accounts AccountRepository, ssh SSHClient, env EnvironmentInspector, git GitClient, mechanism IdentityMechanism) *DiagnosticService {
	return &DiagnosticService{accounts: accounts, ssh: ssh, env: env, git: git, mechanism: mechanism}
}

// Run executes every validation and returns the full ordered list of
// findings.
func (s *DiagnosticService) Run() ([]DiagnosticFinding, error) {
	var findings []DiagnosticFinding

	findings = append(findings, s.checkToolsInstalled()...)

	accounts, err := s.accounts.List()
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}

	findings = append(findings, s.checkKeys(accounts)...)
	findings = append(findings, s.checkAuthentication(accounts)...)
	findings = append(findings, s.checkNoSharedKeys(accounts)...)

	if s.git != nil {
		repoFindings, err := s.checkCurrentRepository(accounts)
		if err != nil {
			return nil, err
		}
		findings = append(findings, repoFindings...)
	}

	findings = append(findings, s.checkGitSSHCommandEnv())

	return findings, nil
}

// checkToolsInstalled is validation #1.
func (s *DiagnosticService) checkToolsInstalled() []DiagnosticFinding {
	var findings []DiagnosticFinding

	if version, err := s.env.GitVersion(); err != nil {
		findings = append(findings, DiagnosticFinding{
			Message: "Git installation",
			Causes:  []string{err.Error()},
			Command: "install Git 2.30 or later",
		})
	} else {
		findings = append(findings, DiagnosticFinding{OK: true, Message: fmt.Sprintf("Git %s installed", version)})
	}

	if s.env.SSHInstalled() {
		findings = append(findings, DiagnosticFinding{OK: true, Message: "SSH installed"})
	} else {
		findings = append(findings, DiagnosticFinding{
			Message: "SSH installation",
			Causes:  []string{"the ssh binary was not found on PATH"},
			Command: "install OpenSSH",
		})
	}

	return findings
}

// checkKeys is validations #2 and #3, folded into one finding per account
// (§13.10's example shows exactly one line per account's key).
func (s *DiagnosticService) checkKeys(accounts []domain.Account) []DiagnosticFinding {
	var findings []DiagnosticFinding
	for _, a := range accounts {
		if a.PrivateKeyPath == "" {
			continue
		}
		if err := s.ssh.ValidateKey(a.PrivateKeyPath); err != nil {
			findings = append(findings, DiagnosticFinding{
				Message: fmt.Sprintf("%s key found (%s)", a.DisplayName, a.PrivateKeyPath),
				Causes:  []string{"the key file is missing or unreadable"},
				Command: fmt.Sprintf("git rodolfo account edit %s", a.ID),
			})
			continue
		}
		cause, fix, err := s.ssh.KeyPermissionIssue(a.PrivateKeyPath)
		if err != nil {
			findings = append(findings, DiagnosticFinding{
				Message: fmt.Sprintf("%s key found (%s)", a.DisplayName, a.PrivateKeyPath),
				Causes:  []string{fmt.Sprintf("could not verify permissions: %v", err)},
			})
			continue
		}
		if cause != "" {
			findings = append(findings, DiagnosticFinding{
				Message: fmt.Sprintf("%s key found (%s)", a.DisplayName, a.PrivateKeyPath),
				Causes:  []string{cause},
				Command: fix,
			})
			continue
		}
		findings = append(findings, DiagnosticFinding{OK: true, Message: fmt.Sprintf("%s key found (%s)", a.DisplayName, a.PrivateKeyPath)})
	}
	return findings
}

// checkAuthentication is validation #4.
func (s *DiagnosticService) checkAuthentication(accounts []domain.Account) []DiagnosticFinding {
	var findings []DiagnosticFinding
	for _, a := range accounts {
		if a.PrivateKeyPath == "" {
			continue
		}
		result, err := s.ssh.TestConnectionWithKey(a.Hostname, a.PrivateKeyPath)
		if err != nil {
			findings = append(findings, DiagnosticFinding{
				Message: fmt.Sprintf("%s authentication", a.DisplayName),
				Causes:  []string{err.Error()},
				Command: fmt.Sprintf("git rodolfo account show %s", a.ID),
			})
			continue
		}
		if result.Success && result.Username == a.ProviderUsername {
			findings = append(findings, DiagnosticFinding{OK: true, Message: fmt.Sprintf("%s authenticates as %s", a.DisplayName, result.Username)})
			continue
		}
		f := DiagnosticFinding{
			Message: fmt.Sprintf("%s authentication failed", a.DisplayName),
			Command: fmt.Sprintf("git rodolfo account show %s", a.ID),
		}
		if result.Success {
			f.Causes = []string{fmt.Sprintf("the key authenticates as %q, not the registered %q", result.Username, a.ProviderUsername)}
		} else {
			f.Causes = []string{
				"the public key has not been added to GitHub",
				"the organization requires SSO authorization for this key",
			}
		}
		findings = append(findings, f)
	}
	return findings
}

// checkNoSharedKeys is validation #5.
func (s *DiagnosticService) checkNoSharedKeys(accounts []domain.Account) []DiagnosticFinding {
	byFingerprint := map[string][]string{}
	for _, a := range accounts {
		if a.PublicKeyFingerprint == "" {
			continue
		}
		byFingerprint[a.PublicKeyFingerprint] = append(byFingerprint[a.PublicKeyFingerprint], a.DisplayName)
	}
	for _, names := range byFingerprint {
		if len(names) > 1 {
			return []DiagnosticFinding{{
				Message: "Shared keys between accounts",
				Causes:  []string{fmt.Sprintf("accounts %v share the same SSH key", names)},
				Command: "git rodolfo account edit <account> --key <a-different-key>",
			}}
		}
	}
	return []DiagnosticFinding{{OK: true, Message: "No shared keys between accounts"}}
}

// checkCurrentRepository is validations #6 and #7 — only run when doctor
// executes inside a repository.
func (s *DiagnosticService) checkCurrentRepository(accounts []domain.Account) ([]DiagnosticFinding, error) {
	var findings []DiagnosticFinding

	accountID, err := s.git.GetLocalConfig("rodolfo.account")
	if errors.Is(err, ErrConfigKeyNotSet) || accountID == "" {
		return findings, nil // not managed here: #6/#7 don't apply
	}
	if err != nil {
		return nil, err
	}

	var assigned *domain.Account
	for i := range accounts {
		if accounts[i].ID == accountID {
			assigned = &accounts[i]
			break
		}
	}
	if assigned == nil {
		findings = append(findings, DiagnosticFinding{
			Message: "Current repository account",
			Causes:  []string{fmt.Sprintf("assigned account %q no longer exists", accountID)},
			Command: "git rodolfo use",
		})
		return findings, nil
	}

	validation, err := s.mechanism.Verify(s.git, *assigned)
	if err != nil {
		return nil, err
	}
	if validation.OK {
		findings = append(findings, DiagnosticFinding{OK: true, Message: fmt.Sprintf("Current repository matches account %q", assigned.DisplayName)})
	} else {
		findings = append(findings, DiagnosticFinding{
			Message: fmt.Sprintf("Current repository matches account %q", assigned.DisplayName),
			Causes:  validation.Issues,
			Command: fmt.Sprintf("git rodolfo use %s", assigned.ID),
		})
	}

	if remoteURL, err := s.git.GetRemoteURL("origin"); err == nil {
		if u, parseErr := domain.ParseRepositoryURL(remoteURL); parseErr == nil {
			if err := ValidateHost(u, remoteURL); err == nil {
				findings = append(findings, DiagnosticFinding{OK: true, Message: fmt.Sprintf("Remote host supported (%s)", u.Host)})
			} else {
				findings = append(findings, DiagnosticFinding{
					Message: "Remote host supported",
					Causes:  []string{fmt.Sprintf("host %q is not supported by Git Rodolfo", u.Host)},
					Command: "point origin at a github.com repository",
				})
			}
		}
	} else if !errors.Is(err, ErrNoRemote) {
		return nil, err
	}

	return findings, nil
}

// checkGitSSHCommandEnv is validation #8: the one failure mode of §12.2's
// chosen mechanism — GIT_SSH_COMMAND in the environment silently overrides
// core.sshCommand.
func (s *DiagnosticService) checkGitSSHCommandEnv() DiagnosticFinding {
	if v := s.env.Getenv("GIT_SSH_COMMAND"); v != "" {
		return DiagnosticFinding{
			Message: "GIT_SSH_COMMAND environment variable",
			Causes:  []string{fmt.Sprintf("GIT_SSH_COMMAND=%q overrides every repository's core.sshCommand", v)},
			Command: "unset GIT_SSH_COMMAND",
		}
	}
	return DiagnosticFinding{OK: true, Message: "GIT_SSH_COMMAND is not set"}
}
