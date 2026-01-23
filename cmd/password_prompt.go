package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

func readPasswordLine(prompt string) (string, error) {
	in := os.Stdin
	out := os.Stderr
	fd := int(in.Fd())
	var tty *os.File

	// If stdin is not a TTY (e.g. systemd), try /dev/tty.
	if !term.IsTerminal(fd) {
		if f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0); err == nil {
			tty = f
			in = f
			out = f
			fd = int(f.Fd())
		} else {
			// Avoid blocking forever on stdin when there is no controlling TTY.
			return "", fmt.Errorf("no TTY available for password prompt")
		}
	}
	if tty != nil {
		defer tty.Close()
	}

	fmt.Fprint(out, prompt)

	if term.IsTerminal(fd) {
		b, err := term.ReadPassword(fd)
		fmt.Fprintln(out)
		return strings.TrimSpace(string(b)), err
	}

	// Fallback: visible input (should be rare).
	r := bufio.NewReader(in)
	s, err := r.ReadString('\n')
	if err != nil {
		// Allow EOF (e.g. piped single line).
		if !errors.Is(err, os.ErrClosed) {
			// continue
		}
	}
	return strings.TrimSpace(s), nil
}

func promptNewPassword() (string, error) {
	p1, err := readPasswordLine("Set admin password: ")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(p1) == "" {
		return "", fmt.Errorf("password cannot be empty")
	}
	p2, err := readPasswordLine("Confirm password: ")
	if err != nil {
		return "", err
	}
	if p1 != p2 {
		return "", fmt.Errorf("passwords do not match")
	}
	return p1, nil
}
