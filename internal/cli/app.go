package cli

import (
	"context"
	"io"
)

// App holds the CLI stream bindings. Keeping them explicit makes commands
// testable without replacing process-wide globals.
type App struct {
	in     io.Reader
	out    io.Writer
	errOut io.Writer
}

// NewApp creates a CLI application bound to the provided streams.
func NewApp(in io.Reader, out, errOut io.Writer) *App {
	return &App{
		in:     in,
		out:    out,
		errOut: errOut,
	}
}

// Execute builds the command tree and runs it with the given arguments.
func (a *App) Execute(ctx context.Context, args []string) error {
	cmd := a.newRootCommand()
	cmd.SetArgs(args)
	cmd.SetIn(a.in)
	cmd.SetOut(a.out)
	cmd.SetErr(a.errOut)

	return cmd.ExecuteContext(ctx)
}
