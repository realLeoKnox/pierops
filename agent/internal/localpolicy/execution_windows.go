//go:build windows

package localpolicy

import (
	"errors"
	"os/exec"
)

func ValidateExecutionUser(name string) error {
	return errors.New("controlled terminal and command execution currently require Unix; Windows monitoring and bounded files remain available")
}
func PrepareCommand(cmd *exec.Cmd, name string) error { return ValidateExecutionUser(name) }
func ExecutionHome(name string) (string, error)       { return "", ValidateExecutionUser(name) }

const nonblockingOpen = 0
