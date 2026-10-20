package cli

import (
	"bytes"
	"context"
	"testing"

	"github.com/spf13/cobra"
)

func executeCommand(t *testing.T, cmd *cobra.Command, args ...string) (string, string, error) {
	t.Helper()

	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(args)

	err := cmd.ExecuteContext(context.Background())

	return out.String(), errOut.String(), err
}
