package utils

import "os/exec"

func RunCmd(args ...string) ([]byte, error) {
	n := len(args)
	if n == 0 {
		return nil, nil
	}

	cmd := args[0]

	if n > 1 {
		args = args[1:]
	} else {
		args = nil
	}

	c := exec.Command(cmd, args...)
	return c.CombinedOutput()
}
