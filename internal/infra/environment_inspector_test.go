package infra

import "testing"

func TestEnvironmentInspector_GitVersion(t *testing.T) {
	requireGitTools(t)
	version, err := NewEnvironmentInspector().GitVersion()
	if err != nil {
		t.Fatalf("GitVersion: %v", err)
	}
	if version == "" {
		t.Fatal("expected a non-empty version string")
	}
}

func TestEnvironmentInspector_SSHInstalled(t *testing.T) {
	requireSSHTools(t)
	if !NewEnvironmentInspector().SSHInstalled() {
		t.Fatal("expected ssh to be detected as installed")
	}
}

func TestEnvironmentInspector_Getenv(t *testing.T) {
	t.Setenv("GIT_RODOLFO_TEST_VAR", "hello")
	if got := NewEnvironmentInspector().Getenv("GIT_RODOLFO_TEST_VAR"); got != "hello" {
		t.Fatalf("got %q", got)
	}
	if got := NewEnvironmentInspector().Getenv("GIT_RODOLFO_TEST_VAR_UNSET"); got != "" {
		t.Fatalf("expected empty string for an unset var, got %q", got)
	}
}
