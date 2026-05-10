package serverworker

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

const defaultMaxDownloadBytes int64 = 2 * 1024 * 1024 * 1024
const processTerminateGrace = 2 * time.Second

type ServerWorker struct {
	options Options
}

type Options struct {
	MaxDownloadBytes    int64
	ProgressBytesStep   int64
	TempDir             string
	ExecTempPattern     string
	DownloadTempPattern string
}

type DownloadableEntry struct {
	Name  string
	IsDir bool
}

type PreparedDownload struct {
	FileName string
	Path     string
	Size     int64
	Cleanup  func() error
}

type DownloadProgress struct {
	Stage      string
	Name       string
	Path       string
	Files      int64
	Bytes      int64
	TotalBytes int64
}

type DownloadProgressFunc func(DownloadProgress)

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

func (sw *ServerWorker) ListCurrentDir() ([]DownloadableEntry, error) {
	entries, err := os.ReadDir(".")
	if err != nil {
		return nil, err
	}

	items := make([]DownloadableEntry, 0, len(entries))
	for _, entry := range entries {
		items = append(items, DownloadableEntry{
			Name:  entry.Name(),
			IsDir: entry.IsDir(),
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].IsDir != items[j].IsDir {
			return items[i].IsDir
		}
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})

	return items, nil
}

func (sw *ServerWorker) PrepareDownload(name string) (*PreparedDownload, error) {
	return sw.PrepareDownloadWithProgress(name, nil)
}

func (sw *ServerWorker) PrepareDownloadWithProgress(name string, progress DownloadProgressFunc) (*PreparedDownload, error) {
	emitDownloadProgress(progress, DownloadProgress{Stage: "resolve_started", Name: name})

	resolvedPath, err := sw.resolveDownloadPath(name)
	if err != nil {
		emitDownloadProgress(progress, DownloadProgress{Stage: "resolve_failed", Name: name})
		return nil, err
	}
	emitDownloadProgress(progress, DownloadProgress{Stage: "resolve_done", Name: name, Path: resolvedPath})

	fileInfo, err := os.Stat(resolvedPath)
	if err != nil {
		return nil, err
	}

	totalBytes, files, err := sw.validateDownloadSize(resolvedPath, progress)
	if err != nil {
		return nil, err
	}
	emitDownloadProgress(progress, DownloadProgress{Stage: "size_check_done", Name: fileInfo.Name(), Path: resolvedPath, Files: files, TotalBytes: totalBytes})

	if !fileInfo.IsDir() {
		emitDownloadProgress(progress, DownloadProgress{Stage: "file_ready", Name: fileInfo.Name(), Path: resolvedPath, Files: files, TotalBytes: totalBytes})
		return &PreparedDownload{
			FileName: fileInfo.Name(),
			Path:     resolvedPath,
			Size:     totalBytes,
			Cleanup:  func() error { return nil },
		}, nil
	}

	archivePath, cleanupArchive, err := sw.archiveDirectory(resolvedPath, fileInfo.Name(), totalBytes, progress)
	if err != nil {
		return nil, err
	}
	archiveInfo, err := os.Stat(archivePath)
	if err != nil {
		_ = cleanupArchive()
		return nil, err
	}
	emitDownloadProgress(progress, DownloadProgress{Stage: "archive_ready", Name: fileInfo.Name() + ".zip", Path: archivePath, Files: files, Bytes: archiveInfo.Size(), TotalBytes: totalBytes})

	return &PreparedDownload{
		FileName: fileInfo.Name() + ".zip",
		Path:     archivePath,
		Size:     archiveInfo.Size(),
		Cleanup:  cleanupArchive,
	}, nil
}

func emitDownloadProgress(progress DownloadProgressFunc, event DownloadProgress) {
	if progress != nil {
		progress(event)
	}
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
	return NewWithOptions(Options{})
}

func NewWithOptions(options Options) (*ServerWorker, error) {
	if options.MaxDownloadBytes == 0 {
		options.MaxDownloadBytes = defaultMaxDownloadBytes
	}
	if options.ProgressBytesStep == 0 {
		options.ProgressBytesStep = 8 * 1024 * 1024
	}
	if options.TempDir == "" {
		options.TempDir = "/tmp"
	}
	if options.ExecTempPattern == "" {
		options.ExecTempPattern = "serverbot-exec-*"
	}
	if options.DownloadTempPattern == "" {
		options.DownloadTempPattern = "serverbot-download-*"
	}

	return &ServerWorker{options: options}, nil
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

	tempDir, err := os.MkdirTemp(se.options.TempDir, se.options.ExecTempPattern)
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

	cmd := exec.Command(bashPath, scriptPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	workingDir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	cmd.Dir = workingDir

	out := &bytes.Buffer{}
	cmd.Stdout = out
	cmd.Stderr = out

	if err := se.runCommand(ctx, cmd); err != nil {
		return se.sanitizeScriptOutput(out.String(), scriptPath), err
	}

	return se.sanitizeScriptOutput(out.String(), scriptPath), nil
}

func (se *ServerWorker) runCommand(ctx context.Context, cmd *exec.Cmd) error {
	if err := cmd.Start(); err != nil {
		return err
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		se.terminateProcessGroup(cmd.Process.Pid, done)
		return errors.New("command terminated")
	}
}

func (se *ServerWorker) terminateProcessGroup(pid int, done <-chan error) {
	if pid <= 0 {
		return
	}

	_ = syscall.Kill(-pid, syscall.SIGTERM)

	timer := time.NewTimer(processTerminateGrace)
	defer timer.Stop()

	select {
	case <-done:
		return
	case <-timer.C:
		_ = syscall.Kill(-pid, syscall.SIGKILL)
		<-done
	}
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

func (se *ServerWorker) resolveDownloadPath(name string) (string, error) {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return "", errors.New("empty file name")
	}

	if filepath.Base(trimmedName) != trimmedName || trimmedName == "." || trimmedName == ".." {
		return "", errors.New("invalid file or directory name")
	}

	workingDir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	resolvedPath := filepath.Join(workingDir, trimmedName)
	_, err = os.Stat(resolvedPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", errors.New("selected file or directory no longer exists")
		}
		return "", err
	}

	return resolvedPath, nil
}

func (se *ServerWorker) validateDownloadSize(path string, progress DownloadProgressFunc) (int64, int64, error) {
	if se.options.MaxDownloadBytes <= 0 {
		return 0, 0, nil
	}

	var totalSize int64
	var files int64
	emitDownloadProgress(progress, DownloadProgress{Stage: "size_check_started", Path: path})
	err := filepath.WalkDir(path, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}

		fileInfo, err := d.Info()
		if err != nil {
			return err
		}

		files++
		totalSize += fileInfo.Size()
		emitDownloadProgress(progress, DownloadProgress{Stage: "size_check_file", Path: path, Files: files, Bytes: fileInfo.Size(), TotalBytes: totalSize})
		if totalSize > se.options.MaxDownloadBytes {
			return fmt.Errorf("selected file or directory is too large for /get: %s exceeds limit %s", formatBytes(totalSize), formatBytes(se.options.MaxDownloadBytes))
		}

		return nil
	})
	if err != nil {
		emitDownloadProgress(progress, DownloadProgress{Stage: "size_check_failed", Path: path, Files: files, TotalBytes: totalSize})
		return totalSize, files, err
	}

	return totalSize, files, nil
}

func formatBytes(size int64) string {
	const mib = 1024 * 1024
	if size >= mib {
		return fmt.Sprintf("%.1f MiB", float64(size)/mib)
	}

	return fmt.Sprintf("%d bytes", size)
}

func (se *ServerWorker) archiveDirectory(directoryPath string, directoryName string, totalBytes int64, progress DownloadProgressFunc) (string, func() error, error) {
	tempDir, err := os.MkdirTemp(se.options.TempDir, se.options.DownloadTempPattern)
	if err != nil {
		return "", nil, err
	}
	cleanup := func() error {
		return os.RemoveAll(tempDir)
	}

	archivePath := filepath.Join(tempDir, directoryName+".zip")
	tempFile, err := os.OpenFile(archivePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		_ = cleanup()
		return "", nil, err
	}

	zipWriter := zip.NewWriter(tempFile)
	emitDownloadProgress(progress, DownloadProgress{Stage: "archive_started", Name: directoryName, Path: archivePath, TotalBytes: totalBytes})

	var archivedFiles int64
	var archivedBytes int64
	walkErr := filepath.WalkDir(directoryPath, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relativePath, err := filepath.Rel(directoryPath, path)
		if err != nil {
			return err
		}

		archiveName := directoryName
		if relativePath != "." {
			archiveName = filepath.ToSlash(filepath.Join(directoryName, relativePath))
		}

		if d.IsDir() {
			if relativePath == "." {
				return nil
			}

			_, err := zipWriter.Create(archiveName + "/")
			return err
		}

		fileInfo, err := d.Info()
		if err != nil {
			return err
		}

		header, err := zip.FileInfoHeader(fileInfo)
		if err != nil {
			return err
		}
		header.Name = archiveName
		header.Method = zip.Deflate

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		progressWriter := &downloadProgressWriter{
			writer:       writer,
			stage:        "archive_bytes",
			name:         archiveName,
			path:         path,
			totalBytes:   totalBytes,
			progress:     progress,
			reportEvery:  se.options.ProgressBytesStep,
			currentFiles: archivedFiles + 1,
			baseBytes:    archivedBytes,
		}
		written, err := io.Copy(progressWriter, file)
		archivedFiles++
		archivedBytes += written
		emitDownloadProgress(progress, DownloadProgress{Stage: "archive_file_done", Name: archiveName, Path: path, Files: archivedFiles, Bytes: archivedBytes, TotalBytes: totalBytes})
		return err
	})

	closeErr := zipWriter.Close()
	fileCloseErr := tempFile.Close()
	if walkErr != nil {
		_ = cleanup()
		emitDownloadProgress(progress, DownloadProgress{Stage: "archive_failed", Name: directoryName, Path: archivePath, Files: archivedFiles, Bytes: archivedBytes, TotalBytes: totalBytes})
		return "", nil, walkErr
	}
	if closeErr != nil {
		_ = cleanup()
		emitDownloadProgress(progress, DownloadProgress{Stage: "archive_failed", Name: directoryName, Path: archivePath, Files: archivedFiles, Bytes: archivedBytes, TotalBytes: totalBytes})
		return "", nil, closeErr
	}
	if fileCloseErr != nil {
		_ = cleanup()
		emitDownloadProgress(progress, DownloadProgress{Stage: "archive_failed", Name: directoryName, Path: archivePath, Files: archivedFiles, Bytes: archivedBytes, TotalBytes: totalBytes})
		return "", nil, fileCloseErr
	}

	emitDownloadProgress(progress, DownloadProgress{Stage: "archive_done", Name: directoryName, Path: archivePath, Files: archivedFiles, Bytes: archivedBytes, TotalBytes: totalBytes})
	return archivePath, cleanup, nil
}

type downloadProgressWriter struct {
	writer       io.Writer
	stage        string
	name         string
	path         string
	totalBytes   int64
	progress     DownloadProgressFunc
	reportEvery  int64
	currentFiles int64
	baseBytes    int64
	written      int64
	nextReport   int64
}

func (w *downloadProgressWriter) Write(p []byte) (int, error) {
	n, err := w.writer.Write(p)
	if n > 0 {
		w.written += int64(n)
		if w.nextReport == 0 {
			w.nextReport = w.reportEvery
		}
		for w.written >= w.nextReport {
			emitDownloadProgress(w.progress, DownloadProgress{
				Stage:      w.stage,
				Name:       w.name,
				Path:       w.path,
				Files:      w.currentFiles,
				Bytes:      w.baseBytes + w.written,
				TotalBytes: w.totalBytes,
			})
			w.nextReport += w.reportEvery
		}
	}

	return n, err
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
