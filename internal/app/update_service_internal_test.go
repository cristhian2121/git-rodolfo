package app

import "testing"

// TestParseChecksumsFile exercises parseChecksumsFile directly (package
// app, not app_test) to cover shasum output quirks that a single
// through-Update() test wouldn't isolate as clearly: the "*" binary-mode
// marker, multiple entries, blank lines, and case-insensitive hex.
func TestParseChecksumsFile(t *testing.T) {
	data := []byte(
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa  git-rodolfo-v0.1.0-darwin-amd64.tar.gz\n" +
			"\n" +
			"BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB  git-rodolfo-v0.1.0-darwin-arm64.tar.gz\n" +
			"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc *git-rodolfo-v0.1.0-linux-amd64.tar.gz\n",
	)

	tests := []struct {
		name     string
		wantHash string
		wantOK   bool
	}{
		{"git-rodolfo-v0.1.0-darwin-amd64.tar.gz", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", true},
		{"git-rodolfo-v0.1.0-darwin-arm64.tar.gz", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", true}, // lowercased
		{"git-rodolfo-v0.1.0-linux-amd64.tar.gz", "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc", true},    // "*" prefix stripped
		{"git-rodolfo-v0.1.0-windows-amd64.tar.gz", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, ok := parseChecksumsFile(data, tt.name)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if hash != tt.wantHash {
				t.Fatalf("hash = %q, want %q", hash, tt.wantHash)
			}
		})
	}
}

func TestParseChecksumsFile_EmptyFile(t *testing.T) {
	if _, ok := parseChecksumsFile([]byte(""), "anything.tar.gz"); ok {
		t.Fatal("expected no match against an empty file")
	}
}

func TestParseChecksumsFile_MalformedLinesIgnored(t *testing.T) {
	data := []byte("this line has only one field\ntoo many fields here to be valid at all\n")
	if _, ok := parseChecksumsFile(data, "anything.tar.gz"); ok {
		t.Fatal("expected malformed lines to be skipped, not matched")
	}
}
