package serverworker

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecCommnad(t *testing.T) {
	worker, err := New()
	require.NoError(t, err)

	result, terminalAsk := worker.Exec(context.Background(), "echo some_text")

	assert.Equal(t, "some_text\n", result)
	assert.Contains(t, terminalAsk, "#")
}

func TestExecutionFail(t *testing.T) {
	worker, err := New()
	require.NoError(t, err)

	result, terminalAsk := worker.Exec(context.Background(), "noSuchCommand arg0 arg1")

	assert.Contains(t, result, "executable file not found")
	assert.Contains(t, terminalAsk, "#")
}

func TestEmptyInput(t *testing.T) {
	worker, err := New()
	require.NoError(t, err)

	result, terminalAsk := worker.Exec(context.Background(), "")

	assert.Contains(t, result, "No command provided")
	assert.Contains(t, terminalAsk, "#")
}

func TestChangeDirectory(t *testing.T) {
	worker, err := New()
	require.NoError(t, err)

	ctx := context.Background()

	currentPath, _ := os.Getwd()
	newCurrentPath, _ := worker.Exec(ctx, "cd .")

	assert.Equal(t, currentPath, newCurrentPath)
}

func TestPipeline(t *testing.T) {
	worker, err := New()
	require.NoError(t, err)

	command := `echo "abcde
	12345
	1a2b3c"
	| grep 1 | grep a`

	result, _ := worker.Exec(context.Background(), command)

	assert.Equal(t, "1a2b3c", result)
}
