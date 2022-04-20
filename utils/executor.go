package utils

import (
	"os/exec"
	"strings"
)

func RunCmd(args ...string) (string, error) {
	n := len(args)
	if n == 0 {
		return "", nil
	}

	cmd := args[0]

	if n > 1 {
		args = args[1:]
	} else {
		args = nil
	}

	c := exec.Command(cmd, args...)
	out, err := c.CombinedOutput()

	return strings.TrimSuffix(string(out), "\n"), err
}
