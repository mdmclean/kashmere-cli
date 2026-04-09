//go:build windows

package cmd

import (
	"bufio"
	"os"
	"strings"
)

func readPasswordFromTerminal() (string, error) {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	return strings.TrimSpace(scanner.Text()), scanner.Err()
}
