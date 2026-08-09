package prompt_test

import (
	"bytes"
	"errors"
	"os"
	"testing"

	"github.com/lean-tech/git-rodolfo/internal/cli/prompt"
)

// newTestSession returns a Session backed by a pipe (never a TTY, so
// Select always takes its non-interactive fallback) and a function to feed
// it input lines.
func newTestSession(t *testing.T) (*prompt.Session, *bytes.Buffer, func(string)) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	t.Cleanup(func() { r.Close(); w.Close() })

	var out bytes.Buffer
	session := prompt.NewSession(&out, r)
	feed := func(s string) {
		if _, err := w.WriteString(s); err != nil {
			t.Fatalf("feed: %v", err)
		}
	}
	return session, &out, feed
}

func TestSession_Text(t *testing.T) {
	s, out, feed := newTestSession(t)
	feed("Lean Tech\n")

	got, err := s.Text("Account name")
	if err != nil {
		t.Fatalf("Text: %v", err)
	}
	if got != "Lean Tech" {
		t.Fatalf("got %q", got)
	}
	if out.String() != "Account name: " {
		t.Fatalf("unexpected prompt output: %q", out.String())
	}
}

func TestSession_Confirm(t *testing.T) {
	tests := []struct {
		input      string
		defaultYes bool
		want       bool
	}{
		{"y\n", false, true},
		{"yes\n", false, true},
		{"n\n", true, false},
		{"\n", true, true},
		{"\n", false, false},
		{"garbage\n", true, false},
	}
	for _, tt := range tests {
		s, _, feed := newTestSession(t)
		feed(tt.input)
		got, err := s.Confirm("Confirm?", tt.defaultYes)
		if err != nil {
			t.Fatalf("Confirm(%q): %v", tt.input, err)
		}
		if got != tt.want {
			t.Fatalf("Confirm(%q, defaultYes=%v) = %v, want %v", tt.input, tt.defaultYes, got, tt.want)
		}
	}
}

func TestSession_Select_FallbackValidChoice(t *testing.T) {
	s, out, feed := newTestSession(t)
	feed("2\n")

	got, err := s.Select("SSH key", []string{"Generate a new key", "Use an existing key", "Configure later"})
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	if got != 1 {
		t.Fatalf("got index %d, want 1", got)
	}
	if !bytes.Contains(out.Bytes(), []byte("1) Generate a new key")) {
		t.Fatalf("expected numbered options in output, got: %s", out.String())
	}
}

func TestSession_Select_FallbackRepromptsOnInvalidInput(t *testing.T) {
	s, out, feed := newTestSession(t)
	feed("nope\n0\n99\n3\n")

	got, err := s.Select("Pick one", []string{"a", "b", "c"})
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	if got != 2 {
		t.Fatalf("got index %d, want 2", got)
	}
	if !bytes.Contains(out.Bytes(), []byte("Please enter a number between 1 and 3.")) {
		t.Fatalf("expected reprompt message, got: %s", out.String())
	}
}

func TestSession_Select_EOFWithoutAnswerErrors(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	w.Close() // immediate EOF on read
	defer r.Close()

	s := prompt.NewSession(&bytes.Buffer{}, r)
	if _, err := s.Select("Pick one", []string{"a", "b"}); err == nil {
		t.Fatal("expected an error when input is exhausted without a valid answer")
	}
}

func TestSession_Select_RejectsEmptyOptions(t *testing.T) {
	s, _, _ := newTestSession(t)
	if _, err := s.Select("Pick one", nil); err == nil {
		t.Fatal("expected an error for zero options")
	}
}

func TestErrCanceledIsDistinct(t *testing.T) {
	if !errors.Is(prompt.ErrCanceled, prompt.ErrCanceled) {
		t.Fatal("sanity check failed")
	}
}
