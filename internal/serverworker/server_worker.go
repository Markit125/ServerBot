package serverworker

import (
	"context"
	"os"
	"os/exec"
	"strings"
)

type ServerWorker struct{}

func (sw *ServerWorker) Exec(ctx context.Context, command string) (string, string) {
	programResult := sw.executeString(ctx, command)
	path, err := os.Getwd()
	if err != nil {
		programResult += "\n" + err.Error()
		return programResult, "#"
	}
	terminalAsk := path + "#"

	return programResult, terminalAsk
}

func New() (*ServerWorker, error) {
	return &ServerWorker{}, nil
}

func (se *ServerWorker) executeString(ctx context.Context, argsStr string) string {
	args := strings.Fields(argsStr)
	if len(args) < 1 {
		return "No command provided"
	}

	command := args[0]
	args = args[1:]

	return se.execute(ctx, command, args)
}

func (se *ServerWorker) execute(ctx context.Context, command string, args []string) string {
	if command == "cd" {
		return se.changeDirectory(args)
	}

	return se.executeCommand(ctx, command, args)
}

func (se *ServerWorker) executeCommand(ctx context.Context, command string, args []string) string {
	cmd := exec.CommandContext(ctx, command, args...)
	stdout, err := cmd.Output()
	if err != nil {
		return err.Error()
	}

	return string(stdout)
}

func (se *ServerWorker) changeDirectory(args []string) string {
	if len(args) < 1 {
		return "Not enough arguments for command 'cd'"
	}

	err := os.Chdir(args[0])
	if err != nil {
		return err.Error()
	}

	path, err := os.Getwd()
	if err != nil {
		return err.Error()
	}

	return path
}
