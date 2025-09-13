package serverworker

import (
	"bytes"
	"context"
	"errors"
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
	out := &bytes.Buffer{}
	commands := strings.Split(argsStr, "|")
	err := se.executeStringCommand(ctx, commands, out)
	if err != nil {
		return err.Error()
	}

	return out.String()
}

func (se *ServerWorker) executeStringCommand(ctx context.Context, commands []string, buffer *bytes.Buffer) error {
	if len(commands) == 0 {
		return nil
	}

	args := strings.Fields(commands[len(commands)-1])
	if len(args) == 0 {
		return errors.New("no command provided")
	}

	command := args[0]
	args = args[1:]

	return se.execute(ctx, command, args, commands[:len(commands)-1], buffer)
}

func (se *ServerWorker) execute(ctx context.Context, command string, args []string, commands []string, buffer *bytes.Buffer) error {
	if command == "cd" {
		return se.changeDirectory(args)
	}

	return se.executeCommand(ctx, command, args, commands, buffer)
}

func (se *ServerWorker) executeCommand(ctx context.Context, command string, args []string, commands []string, buffer *bytes.Buffer) error {
	err := se.executeStringCommand(ctx, commands, buffer)
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Stdout = buffer
	cmd.Stdin = buffer

	if err := cmd.Start(); err != nil {
		return err
	}

	if err := cmd.Wait(); err != nil {
		return err
	}

	return nil
}

func (se *ServerWorker) changeDirectory(args []string) error {
	if len(args) < 1 {
		return errors.New("not enough arguments for command 'cd'")
	}

	err := os.Chdir(args[0])
	if err != nil {
		return err
	}

	return nil
}
