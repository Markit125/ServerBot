package serverworker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type ServerWorker struct{}

func (sw *ServerWorker) Exec(ctx context.Context, command string) (string, string) {
	programResult, err := sw.execute(ctx, command)
	if err != nil {
		if programResult != "" {
			programResult += "\n"
		}
		programResult += err.Error()
	}

	return sw.resultWithPrompt(programResult)
}

func (sw *ServerWorker) TerminalAsk() string {
	_, terminalAsk := sw.resultWithPrompt("")
	return terminalAsk
}

func (sw *ServerWorker) SaveUploadedFile(fileName string, content io.Reader) (string, error) {
	workingDir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	sanitizedFileName := sw.sanitizeUploadedFileName(fileName)
	fileBaseName, fileExtension := sw.splitFileName(sanitizedFileName)

	for duplicateIndex := 0; ; duplicateIndex++ {
		candidateName := sw.buildUploadedFileName(fileBaseName, fileExtension, duplicateIndex)
		candidatePath := filepath.Join(workingDir, candidateName)

		file, err := os.OpenFile(candidatePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			if errors.Is(err, fs.ErrExist) {
				continue
			}
			return "", err
		}

		_, copyErr := io.Copy(file, content)
		closeErr := file.Close()
		if copyErr != nil {
			_ = os.Remove(candidatePath)
			return "", copyErr
		}
		if closeErr != nil {
			_ = os.Remove(candidatePath)
			return "", closeErr
		}

		return candidatePath, nil
	}
}

func (sw *ServerWorker) SaveTelegramLocalFile(sourcePath string, targetFileName string) (string, error) {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return "", err
	}
	defer sourceFile.Close()

	return sw.SaveUploadedFile(targetFileName, sourceFile)
}

func (sw *ServerWorker) resultWithPrompt(programResult string) (string, string) {
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

func (se *ServerWorker) execute(ctx context.Context, command string) (string, error) {
	if se.isChangeDirectoryCommand(command) {
		return "", se.changeDirectory(command)
	}

	return se.executeScript(ctx, command)
}

func (se *ServerWorker) executeScript(ctx context.Context, command string) (string, error) {
	if strings.TrimSpace(command) == "" {
		return "", errors.New("no command provided")
	}

	tempDir, err := os.MkdirTemp("/tmp", "server-bot-exec-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tempDir)

	scriptPath := filepath.Join(tempDir, "exec.sh")
	if err := os.WriteFile(scriptPath, []byte(command+"\n"), 0o600); err != nil {
		return "", err
	}

	bashPath, err := se.findBashPath()
	if err != nil {
		return "", err
	}

	cmd := exec.CommandContext(ctx, bashPath, scriptPath)

	workingDir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	cmd.Dir = workingDir

	out := &bytes.Buffer{}
	cmd.Stdout = out
	cmd.Stderr = out

	if err := cmd.Run(); err != nil {
		return se.sanitizeScriptOutput(out.String(), scriptPath), err
	}

	return se.sanitizeScriptOutput(out.String(), scriptPath), nil
}

func (se *ServerWorker) findBashPath() (string, error) {
	return exec.LookPath("bash")
}

func (se *ServerWorker) sanitizeUploadedFileName(fileName string) string {
	sanitizedFileName := filepath.Base(strings.TrimSpace(fileName))
	if sanitizedFileName == "." || sanitizedFileName == string(filepath.Separator) || sanitizedFileName == "" {
		return "uploaded_file"
	}

	return sanitizedFileName
}

func (se *ServerWorker) splitFileName(fileName string) (string, string) {
	if strings.HasPrefix(fileName, ".") && strings.Count(fileName, ".") == 1 {
		return fileName, ""
	}

	fileExtension := filepath.Ext(fileName)
	fileBaseName := strings.TrimSuffix(fileName, fileExtension)
	if fileBaseName == "" {
		fileBaseName = "uploaded_file"
	}

	return fileBaseName, fileExtension
}

func (se *ServerWorker) buildUploadedFileName(fileBaseName string, fileExtension string, duplicateIndex int) string {
	if duplicateIndex == 0 {
		return fileBaseName + fileExtension
	}

	return fmt.Sprintf("%s_%d%s", fileBaseName, duplicateIndex, fileExtension)
}

func (se *ServerWorker) sanitizeScriptOutput(output string, scriptPath string) string {
	sanitizedOutput := strings.ReplaceAll(output, scriptPath, "terminal input")
	return strings.TrimRight(sanitizedOutput, "\r\n")
}

func (se *ServerWorker) isChangeDirectoryCommand(command string) bool {
	trimmedCommand := strings.TrimSpace(command)
	if trimmedCommand == "" || strings.Contains(trimmedCommand, "\n") {
		return false
	}

	commandParts := strings.Fields(trimmedCommand)
	return len(commandParts) > 0 && commandParts[0] == "cd"
}

func (se *ServerWorker) changeDirectory(command string) error {
	commandParts := strings.Fields(strings.TrimSpace(command))
	if len(commandParts) != 2 {
		return errors.New("cd accepts only one path argument")
	}

	return os.Chdir(commandParts[1])
}
