package infra

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/lean-tech/git-rodolfo/internal/domain"
)

func TestDefaultConfigPath_HonorsXDGConfigHome(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/xdg/config")
	got, err := DefaultConfigPath()
	if err != nil {
		t.Fatalf("DefaultConfigPath: %v", err)
	}
	want := filepath.Join("/xdg/config", "git-rodolfo", "config.json")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestDefaultConfigPath_FallsBackToHomeConfig(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	got, err := DefaultConfigPath()
	if err != nil {
		t.Fatalf("DefaultConfigPath: %v", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}
	want := filepath.Join(home, ".config", "git-rodolfo", "config.json")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func newTestAccount(id string) domain.Account {
	return domain.Account{
		ID:                   id,
		DisplayName:          id,
		GitName:              "Test User",
		GitEmail:             id + "@example.com",
		Provider:             domain.ProviderGitHub,
		ProviderUsername:     id,
		Hostname:             "github.com",
		SSHUser:              "git",
		PrivateKeyPath:       "~/.ssh/" + id,
		PublicKeyPath:        "~/.ssh/" + id + ".pub",
		PublicKeyFingerprint: "SHA256:" + id,
		AuthenticationStatus: domain.AuthenticationStatusUnverified,
	}
}

// TestConfigRepository_Save_CreatesMissingConfigDirectory is a regression
// test: on a fresh install, ~/.config/git-rodolfo/ doesn't exist yet, only
// ~/.config/ does. Save must create it rather than fail acquiring the lock
// file in a directory that isn't there.
func TestConfigRepository_Save_CreatesMissingConfigDirectory(t *testing.T) {
	parent := t.TempDir() // stands in for an existing ~/.config
	path := filepath.Join(parent, "git-rodolfo", "config.json")
	repo := NewConfigRepository(path)

	if err := repo.Save(newTestAccount("lean-tech")); err != nil {
		t.Fatalf("save into a not-yet-created directory: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config file was not created: %v", err)
	}
}

func TestConfigRepository_SaveListFindUpdateDelete(t *testing.T) {
	dir := t.TempDir()
	repo := NewConfigRepository(filepath.Join(dir, "config.json"))

	if accounts, err := repo.List(); err != nil || len(accounts) != 0 {
		t.Fatalf("expected empty list on fresh install, got %v, err %v", accounts, err)
	}

	acc := newTestAccount("lean-tech")
	if err := repo.Save(acc); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := repo.Save(acc); err == nil {
		t.Fatal("expected error saving duplicate ID, got nil")
	}

	found, err := repo.FindByID("lean-tech")
	if err != nil || found == nil {
		t.Fatalf("find by id: %v, %v", found, err)
	}
	if found.GitEmail != acc.GitEmail {
		t.Fatalf("unexpected account: %+v", found)
	}

	byFP, err := repo.FindByFingerprint("SHA256:lean-tech")
	if err != nil || byFP == nil {
		t.Fatalf("find by fingerprint: %v, %v", byFP, err)
	}

	if missing, err := repo.FindByID("nope"); err != nil || missing != nil {
		t.Fatalf("expected nil, nil for missing account, got %v, %v", missing, err)
	}

	acc.GitEmail = "updated@example.com"
	if err := repo.Update(acc); err != nil {
		t.Fatalf("update: %v", err)
	}
	found, _ = repo.FindByID("lean-tech")
	if found.GitEmail != "updated@example.com" {
		t.Fatalf("update did not persist: %+v", found)
	}

	if err := repo.Update(newTestAccount("ghost")); err == nil {
		t.Fatal("expected error updating nonexistent account")
	}

	if err := repo.Delete("lean-tech"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if accounts, _ := repo.List(); len(accounts) != 0 {
		t.Fatalf("expected empty list after delete, got %v", accounts)
	}
	if err := repo.Delete("lean-tech"); err == nil {
		t.Fatal("expected error deleting already-deleted account")
	}
}

// TestConfigRepository_ConcurrentSavesDoNotRace is the RNF-06 test: many
// concurrent writers, each adding a distinct account, must all succeed
// without losing updates or corrupting the file — the file lock in
// withLock must serialize the read-modify-write cycles. Run with -race to
// also catch in-process data races.
func TestConfigRepository_ConcurrentSavesDoNotRace(t *testing.T) {
	dir := t.TempDir()
	repo := NewConfigRepository(filepath.Join(dir, "config.json"))

	const n = 25
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = repo.Save(newTestAccount(fmt.Sprintf("account-%02d", i)))
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("goroutine %d: save failed: %v", i, err)
		}
	}

	accounts, err := repo.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(accounts) != n {
		t.Fatalf("expected %d accounts (no lost updates), got %d", n, len(accounts))
	}

	seen := map[string]bool{}
	for _, a := range accounts {
		if seen[a.ID] {
			t.Fatalf("duplicate account ID in final config: %s", a.ID)
		}
		seen[a.ID] = true
	}
}
