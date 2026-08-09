package cli

import (
	"os/exec"
	"runtime"
	"strings"
)

// copyToClipboard tries the platform's clipboard tool. It returns an error
// if none is available, so the caller can fall back to printing the text
// for the user to copy by hand.
func copyToClipboard(text string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "windows":
		cmd = exec.Command("clip")
	default:
		if _, err := exec.LookPath("xclip"); err == nil {
			cmd = exec.Command("xclip", "-selection", "clipboard")
		} else if _, err := exec.LookPath("xsel"); err == nil {
			cmd = exec.Command("xsel", "--clipboard", "--input")
		}
	}
	if cmd == nil {
		return errNoClipboardTool
	}
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

var errNoClipboardTool = clipboardError("no clipboard tool available on this system")

type clipboardError string

func (e clipboardError) Error() string { return string(e) }
