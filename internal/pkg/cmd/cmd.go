package cmd

import (
	"fmt"
	"os/exec"
)

// Exec execute shell command
func Exec(name string, arg ...string) ([]byte, error) {
	cmd := exec.Command(name, arg...)
	result, err := cmd.CombinedOutput()
	if err != nil {
		return result, err
	}

	// log.Debug().Msgf("cmd stdout: %s", string(result))

	return result, nil
}

func Run(cmd string) ([]byte, error) {
	return Exec("bash", "-c", cmd)
}

func Runf(format string, a ...any) ([]byte, error) {
	return Run(fmt.Sprintf(format, a...))
}
