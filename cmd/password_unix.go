//go:build !windows

package cmd

import (
	"fmt"
	"os"

	"golang.org/x/term"
)

func readPasswordFromTerminal() (string, error) {
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr) // newline after hidden input
	return string(b), err
}
