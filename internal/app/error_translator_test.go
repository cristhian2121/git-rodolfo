package app_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/lean-tech/git-rodolfo/internal/app"
	"github.com/lean-tech/git-rodolfo/internal/domain"
	"github.com/lean-tech/git-rodolfo/internal/infra/fakes"
)

const repositoryNotFoundOutput = "ERROR: Repository not found.\nfatal: Could not read from remote repository."

func TestErrorTranslator_RepositoryNotFound_WithSuggestion(t *testing.T) {
	repo := fakes.NewFakeAccountRepository(
		domain.Account{ID: "personal", DisplayName: "Personal", ProviderUsername: "cristhiandelgado"},
		domain.Account{ID: "lean-tech", DisplayName: "Lean Tech"},
	)
	translator := app.NewErrorTranslator(repo)
	gitErr := &app.GitCommandError{Output: repositoryNotFoundOutput, Err: errors.New("exit status 128")}

	msg, ok := translator.Translate(gitErr, app.OperationContext{
		Account:   domain.Account{ID: "personal", DisplayName: "Personal", ProviderUsername: "cristhiandelgado"},
		RemoteURL: "git@github.com:lean-tech/project.git",
	})
	if !ok {
		t.Fatal("expected Repository not found to be translated")
	}
	for _, want := range []string{
		"GitHub says the repository was not found.",
		"You authenticated as: cristhiandelgado (Personal)",
		"The repository belongs to: lean-tech",
		`You have an account for that organization: "Lean Tech"`,
		"git rodolfo clone git@github.com:lean-tech/project.git --account lean-tech",
		"Original error:",
		repositoryNotFoundOutput,
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("message missing %q:\n%s", want, msg)
		}
	}
}

func TestErrorTranslator_RepositoryNotFound_NoSuggestionAvailable(t *testing.T) {
	repo := fakes.NewFakeAccountRepository(domain.Account{ID: "personal", DisplayName: "Personal"})
	translator := app.NewErrorTranslator(repo)
	gitErr := &app.GitCommandError{Output: repositoryNotFoundOutput}

	msg, ok := translator.Translate(gitErr, app.OperationContext{
		Account:   domain.Account{ID: "personal", DisplayName: "Personal", ProviderUsername: "cristhiandelgado"},
		RemoteURL: "git@github.com:some-other-org/project.git",
	})
	if !ok {
		t.Fatal("expected translation")
	}
	if strings.Contains(msg, "You have an account for that organization") {
		t.Fatalf("should not suggest an account that doesn't exist:\n%s", msg)
	}
}

func TestErrorTranslator_PermissionDenied_OnlyAppendsCommand(t *testing.T) {
	translator := app.NewErrorTranslator(fakes.NewFakeAccountRepository())
	original := "git@github.com: Permission denied (publickey).\nfatal: Could not read from remote repository."
	gitErr := &app.GitCommandError{Output: original}

	msg, ok := translator.Translate(gitErr, app.OperationContext{})
	if !ok {
		t.Fatal("expected translation")
	}
	if !strings.HasPrefix(msg, original) {
		t.Fatalf("expected the original message preserved verbatim at the start, got:\n%s", msg)
	}
	if !strings.Contains(msg, "git rodolfo doctor") {
		t.Fatalf("expected a suggested command, got:\n%s", msg)
	}
}

func TestErrorTranslator_OtherErrors_PassThroughUnmodified(t *testing.T) {
	translator := app.NewErrorTranslator(fakes.NewFakeAccountRepository())
	gitErr := &app.GitCommandError{Output: "fatal: unable to access 'https://github.com/x/y.git/': Could not resolve host"}

	_, ok := translator.Translate(gitErr, app.OperationContext{})
	if ok {
		t.Fatal("expected an unrelated Git error to pass through untranslated")
	}
}

func TestErrorTranslator_NonGitCommandError_PassesThrough(t *testing.T) {
	translator := app.NewErrorTranslator(fakes.NewFakeAccountRepository())
	_, ok := translator.Translate(errors.New("some other error"), app.OperationContext{})
	if ok {
		t.Fatal("expected a non-GitCommandError to never be translated")
	}
}
