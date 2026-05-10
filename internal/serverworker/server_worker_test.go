package serverworker

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecCommnad(t *testing.T) {
	worker, err := New()
	require.NoError(t, err)

	result, terminalAsk := worker.Exec(context.Background(), "printf some_text")

	assert.Equal(t, "some_text", result)
	assert.Contains(t, terminalAsk, "#")
}

func TestExecutionFail(t *testing.T) {
	worker, err := New()
	require.NoError(t, err)

	result, terminalAsk := worker.Exec(context.Background(), "noSuchCommand arg0 arg1")

	assert.Contains(t, result, "command not found")
	assert.Contains(t, result, "exit status 127")
	assert.Contains(t, terminalAsk, "#")
}

func TestExecutionSyntaxErrorDoesNotExposeTemporaryScriptPath(t *testing.T) {
	worker, err := New()
	require.NoError(t, err)

	result, terminalAsk := worker.Exec(context.Background(), `cat "`)

	assert.Contains(t, result, "terminal input: line 1")
	assert.Contains(t, result, `unexpected EOF while looking for matching`)
	assert.NotContains(t, result, "/tmp/")
	assert.NotContains(t, result, "exec.sh")
	assert.Contains(t, result, "exit status 2")
	assert.Contains(t, terminalAsk, "#")
}

func TestEmptyInput(t *testing.T) {
	worker, err := New()
	require.NoError(t, err)

	result, terminalAsk := worker.Exec(context.Background(), "")

	assert.Contains(t, result, "no command provided")
	assert.Contains(t, terminalAsk, "#")
}

func TestChangeDirectory(t *testing.T) {
	worker, err := New()
	require.NoError(t, err)

	ctx := context.Background()

	tempDir := t.TempDir()
	currentDir, err := os.Getwd()
	require.NoError(t, err)

	require.NoError(t, os.Chdir(tempDir))
	t.Cleanup(func() {
		_ = os.Chdir(currentDir)
	})

	_, terminalAsk := worker.Exec(ctx, "cd .")

	assert.Equal(t, tempDir+"#", terminalAsk)
}

func TestChangeDirectoryToParent(t *testing.T) {
	worker, err := New()
	require.NoError(t, err)

	ctx := context.Background()
	parentDir := t.TempDir()
	childDir := filepath.Join(parentDir, "child")
	require.NoError(t, os.Mkdir(childDir, 0o755))

	currentDir, err := os.Getwd()
	require.NoError(t, err)

	require.NoError(t, os.Chdir(childDir))
	t.Cleanup(func() {
		_ = os.Chdir(currentDir)
	})

	result, terminalAsk := worker.Exec(ctx, "cd ..")

	assert.Equal(t, "", result)
	assert.Equal(t, parentDir+"#", terminalAsk)
}

func TestPipeline(t *testing.T) {
	worker, err := New()
	require.NoError(t, err)

	command := `printf "abcde\n12345\n1a2b3c\n" | grep 1 | grep a`

	result, _ := worker.Exec(context.Background(), command)

	assert.Equal(t, "1a2b3c", result)
}

func TestExecTerminatesCancelledCommand(t *testing.T) {
	worker, err := New()
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	startedAt := time.Now()
	result, terminalAsk := worker.Exec(ctx, "sleep 30")

	assert.Less(t, time.Since(startedAt), processTerminateGrace+time.Second)
	assert.Contains(t, result, "command terminated")
	assert.Contains(t, terminalAsk, "#")
}

func TestSaveUploadedFileAddsDuplicateSuffix(t *testing.T) {
	worker, err := New()
	require.NoError(t, err)

	tempDir := t.TempDir()
	currentDir, err := os.Getwd()
	require.NoError(t, err)

	require.NoError(t, os.Chdir(tempDir))
	t.Cleanup(func() {
		_ = os.Chdir(currentDir)
	})

	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "report.txt"), []byte("existing"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "report_1.txt"), []byte("existing"), 0o600))

	savedPath, err := worker.SaveUploadedFile("report.txt", bytes.NewBufferString("fresh"))
	require.NoError(t, err)

	assert.Equal(t, filepath.Join(tempDir, "report_2.txt"), savedPath)

	savedContent, err := os.ReadFile(savedPath)
	require.NoError(t, err)
	assert.Equal(t, "fresh", string(savedContent))
}

func TestSaveUploadedFileSanitizesFileName(t *testing.T) {
	worker, err := New()
	require.NoError(t, err)

	tempDir := t.TempDir()
	currentDir, err := os.Getwd()
	require.NoError(t, err)

	require.NoError(t, os.Chdir(tempDir))
	t.Cleanup(func() {
		_ = os.Chdir(currentDir)
	})

	savedPath, err := worker.SaveUploadedFile("../secret.txt", bytes.NewBufferString("payload"))
	require.NoError(t, err)

	assert.Equal(t, filepath.Join(tempDir, "secret.txt"), savedPath)
}

func TestSaveTelegramLocalFileCopiesIntoCurrentDirectory(t *testing.T) {
	worker, err := New()
	require.NoError(t, err)

	sourceDir := t.TempDir()
	targetDir := t.TempDir()
	currentDir, err := os.Getwd()
	require.NoError(t, err)

	sourcePath := filepath.Join(sourceDir, "track.mp3")
	require.NoError(t, os.WriteFile(sourcePath, []byte("audio-data"), 0o600))

	require.NoError(t, os.Chdir(targetDir))
	t.Cleanup(func() {
		_ = os.Chdir(currentDir)
	})

	savedPath, err := worker.SaveTelegramLocalFile(sourcePath, "track.mp3")
	require.NoError(t, err)

	assert.Equal(t, filepath.Join(targetDir, "track.mp3"), savedPath)

	savedContent, err := os.ReadFile(savedPath)
	require.NoError(t, err)
	assert.Equal(t, "audio-data", string(savedContent))
}

func TestExecCleansTemporaryScript(t *testing.T) {
	worker, err := New()
	require.NoError(t, err)

	tempDir := t.TempDir()
	currentDir, err := os.Getwd()
	require.NoError(t, err)

	require.NoError(t, os.Chdir(tempDir))
	t.Cleanup(func() {
		_ = os.Chdir(currentDir)
	})

	result, terminalAsk := worker.Exec(context.Background(), "pwd\necho script-result")

	assert.Contains(t, result, "script-result")
	assert.Contains(t, terminalAsk, tempDir)

	matches, err := filepath.Glob(filepath.Join(tempDir, "exec-*"))
	require.NoError(t, err)
	assert.Empty(t, matches)
}

func TestListCurrentDirReturnsDirectoriesFirst(t *testing.T) {
	worker, err := New()
	require.NoError(t, err)

	tempDir := t.TempDir()
	currentDir, err := os.Getwd()
	require.NoError(t, err)

	require.NoError(t, os.Chdir(tempDir))
	t.Cleanup(func() {
		_ = os.Chdir(currentDir)
	})

	require.NoError(t, os.Mkdir("beta", 0o755))
	require.NoError(t, os.Mkdir("Alpha", 0o755))
	require.NoError(t, os.WriteFile("zeta.txt", []byte("z"), 0o600))
	require.NoError(t, os.WriteFile("beta.txt", []byte("b"), 0o600))

	entries, err := worker.ListCurrentDir()
	require.NoError(t, err)

	assert.Equal(t, []DownloadableEntry{
		{Name: "Alpha", IsDir: true},
		{Name: "beta", IsDir: true},
		{Name: "beta.txt", IsDir: false},
		{Name: "zeta.txt", IsDir: false},
	}, entries)
}

func TestPrepareDownloadReturnsFileAsIs(t *testing.T) {
	worker, err := New()
	require.NoError(t, err)

	tempDir := t.TempDir()
	currentDir, err := os.Getwd()
	require.NoError(t, err)

	require.NoError(t, os.Chdir(tempDir))
	t.Cleanup(func() {
		_ = os.Chdir(currentDir)
	})

	filePath := filepath.Join(tempDir, "report.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("payload"), 0o600))

	preparedDownload, err := worker.PrepareDownload("report.txt")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = preparedDownload.Cleanup()
	})

	assert.Equal(t, "report.txt", preparedDownload.FileName)
	assert.Equal(t, filePath, preparedDownload.Path)
}

func TestPrepareDownloadArchivesDirectory(t *testing.T) {
	worker, err := New()
	require.NoError(t, err)

	tempDir := t.TempDir()
	currentDir, err := os.Getwd()
	require.NoError(t, err)

	require.NoError(t, os.Chdir(tempDir))
	t.Cleanup(func() {
		_ = os.Chdir(currentDir)
	})

	require.NoError(t, os.MkdirAll(filepath.Join("logs", "nested"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join("logs", "app.log"), []byte("line-1"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join("logs", "nested", "trace.txt"), []byte("trace"), 0o600))

	preparedDownload, err := worker.PrepareDownload("logs")
	require.NoError(t, err)

	assert.Equal(t, "logs.zip", preparedDownload.FileName)
	assert.Equal(t, "logs.zip", filepath.Base(preparedDownload.Path))

	archiveReader, err := zip.OpenReader(preparedDownload.Path)
	require.NoError(t, err)
	defer archiveReader.Close()

	archiveEntries := make([]string, 0, len(archiveReader.File))
	for _, file := range archiveReader.File {
		archiveEntries = append(archiveEntries, file.Name)
	}

	assert.Contains(t, archiveEntries, "logs/app.log")
	assert.Contains(t, archiveEntries, "logs/nested/")
	assert.Contains(t, archiveEntries, "logs/nested/trace.txt")

	require.NoError(t, preparedDownload.Cleanup())
	_, err = os.Stat(preparedDownload.Path)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestPrepareDownloadRejectsTraversal(t *testing.T) {
	worker, err := New()
	require.NoError(t, err)

	_, err = worker.PrepareDownload("../secret.txt")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid file or directory name")
}

func TestPrepareDownloadRejectsFileAboveLimit(t *testing.T) {
	worker, err := NewWithOptions(Options{MaxDownloadBytes: 4})
	require.NoError(t, err)

	tempDir := t.TempDir()
	currentDir, err := os.Getwd()
	require.NoError(t, err)

	require.NoError(t, os.Chdir(tempDir))
	t.Cleanup(func() {
		_ = os.Chdir(currentDir)
	})

	require.NoError(t, os.WriteFile("large.txt", []byte("payload"), 0o600))

	_, err = worker.PrepareDownload("large.txt")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too large")
	assert.Contains(t, err.Error(), "exceeds limit")
}

func TestPrepareDownloadRejectsDirectoryAboveLimit(t *testing.T) {
	worker, err := NewWithOptions(Options{MaxDownloadBytes: 8})
	require.NoError(t, err)

	tempDir := t.TempDir()
	currentDir, err := os.Getwd()
	require.NoError(t, err)

	require.NoError(t, os.Chdir(tempDir))
	t.Cleanup(func() {
		_ = os.Chdir(currentDir)
	})

	require.NoError(t, os.Mkdir("logs", 0o755))
	require.NoError(t, os.WriteFile(filepath.Join("logs", "a.log"), []byte("12345"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join("logs", "b.log"), []byte("67890"), 0o600))

	_, err = worker.PrepareDownload("logs")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too large")
	assert.Contains(t, err.Error(), "exceeds limit")
}
