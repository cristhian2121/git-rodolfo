// Package prompt implements the small set of interactive widgets the
// account wizards need (PRD §13.1: navigate with arrows, type to filter,
// Enter to select, Ctrl+C to cancel), plus their non-interactive
// fallbacks. A single Session shares one buffered reader across all
// widgets so a wizard can freely mix Text/Confirm/Select prompts without
// losing bytes buffered ahead of a line boundary.
package prompt

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"
)

// ErrCanceled is returned by Select when the user cancels with Ctrl+C.
var ErrCanceled = errors.New("canceled")

// Session is the shared input/output for one interactive command.
type Session struct {
	Out io.Writer
	In  *bufio.Reader
	// tty is the file backing In, used for raw-mode/terminal-size
	// operations. It may be a non-terminal (e.g. a pipe in tests), in
	// which case Select transparently falls back to a numbered prompt.
	tty *os.File
}

// NewSession builds a Session over out/in. Pass os.Stdout/os.Stdin for a
// real interactive command; any *os.File works for tests (e.g. one end of
// os.Pipe), and Select detects when it isn't a real terminal.
func NewSession(out io.Writer, in *os.File) *Session {
	return &Session{Out: out, In: bufio.NewReader(in), tty: in}
}

// Text asks a free-text question and returns the trimmed answer.
func (s *Session) Text(label string) (string, error) {
	fmt.Fprintf(s.Out, "%s: ", label)
	line, err := s.In.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// Confirm asks a yes/no question; a bare Enter answers defaultYes.
// Anything other than a recognized yes/no is treated as "no" — the safe
// default for the confirmations this tool uses (RNF-01: confirm before
// destructive operations).
func (s *Session) Confirm(label string, defaultYes bool) (bool, error) {
	suffix := "[y/N]"
	if defaultYes {
		suffix = "[Y/n]"
	}
	fmt.Fprintf(s.Out, "%s %s ", label, suffix)
	line, err := s.In.ReadString('\n')
	if err != nil && err != io.EOF {
		return false, err
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "":
		return defaultYes, nil
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}

// Select shows label and options, letting the user choose one, and returns
// its index. On a real terminal, it's arrow-key navigable with type-to-
// filter; otherwise (piped input, no TTY) it falls back to a numbered list.
func (s *Session) Select(label string, options []string) (int, error) {
	if len(options) == 0 {
		return 0, fmt.Errorf("select %q: no options given", label)
	}
	if s.tty == nil || !term.IsTerminal(int(s.tty.Fd())) {
		return s.selectFallback(label, options)
	}
	return s.selectInteractive(label, options)
}

func (s *Session) selectFallback(label string, options []string) (int, error) {
	fmt.Fprintln(s.Out, label)
	for i, opt := range options {
		fmt.Fprintf(s.Out, "  %d) %s\n", i+1, opt)
	}
	for {
		fmt.Fprint(s.Out, "Enter a number: ")
		line, err := s.In.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return 0, fmt.Errorf("no input available to select an option")
			}
			return 0, err
		}
		n, convErr := strconv.Atoi(strings.TrimSpace(line))
		if convErr == nil && n >= 1 && n <= len(options) {
			return n - 1, nil
		}
		fmt.Fprintf(s.Out, "Please enter a number between 1 and %d.\n", len(options))
	}
}

func (s *Session) selectInteractive(label string, options []string) (int, error) {
	fd := int(s.tty.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return s.selectFallback(label, options)
	}
	defer term.Restore(fd, oldState)

	query := ""
	selected := 0
	linesDrawn := 0

	matching := func() []int {
		if query == "" {
			all := make([]int, len(options))
			for i := range options {
				all[i] = i
			}
			return all
		}
		var idx []int
		q := strings.ToLower(query)
		for i, opt := range options {
			if strings.Contains(strings.ToLower(opt), q) {
				idx = append(idx, i)
			}
		}
		return idx
	}

	redraw := func() []int {
		idx := matching()
		if selected >= len(idx) {
			selected = len(idx) - 1
		}
		if selected < 0 {
			selected = 0
		}

		if linesDrawn > 0 {
			fmt.Fprintf(s.Out, "\x1b[%dA", linesDrawn)
		}
		header := label
		if query != "" {
			header += "  (filter: " + query + ")"
		}
		fmt.Fprintf(s.Out, "\r%s\x1b[K\r\n", header)
		for pos, i := range idx {
			marker := "  "
			if pos == selected {
				marker = "❯ " // ❯
			}
			fmt.Fprintf(s.Out, "\r%s%s\x1b[K\r\n", marker, options[i])
		}
		linesDrawn = len(idx) + 1
		return idx
	}

	idx := redraw()
	for {
		b, err := s.In.ReadByte()
		if err != nil {
			return 0, err
		}
		switch {
		case b == 3: // Ctrl+C
			fmt.Fprint(s.Out, "\r\n")
			return 0, ErrCanceled
		case b == '\r' || b == '\n':
			fmt.Fprint(s.Out, "\r\n")
			if len(idx) == 0 {
				continue
			}
			return idx[selected], nil
		case b == 0x7f || b == 0x08: // backspace
			if len(query) > 0 {
				query = query[:len(query)-1]
				selected = 0
			}
			idx = redraw()
		case b == 0x1b: // escape sequence, e.g. arrow keys: ESC [ A/B
			b2, err := s.In.ReadByte()
			if err != nil || b2 != '[' {
				continue
			}
			b3, err := s.In.ReadByte()
			if err != nil {
				continue
			}
			switch b3 {
			case 'A': // up
				if selected > 0 {
					selected--
				}
			case 'B': // down
				if selected < len(idx)-1 {
					selected++
				}
			}
			idx = redraw()
		case b >= 0x20 && b < 0x7f: // printable — extends the filter
			query += string(b)
			selected = 0
			idx = redraw()
		}
	}
}
