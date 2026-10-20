// Command mcjava-manager is a standalone CLI for managing content in Minecraft
// Java instances, including mods, shader packs, resource packs, and worlds.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"

	"github.com/wedemmmdev/mcjava-manager/internal/cli"
	"github.com/wedemmmdev/mcjava-manager/internal/netdns"
)

func init() {
	// Android/Shizuku may not allow the Go resolver to reach the system DNS
	// configuration, so configure public DNS fallbacks before any network use.
	netdns.Install()
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	app := cli.NewApp(os.Stdin, os.Stdout, os.Stderr)
	if err := app.Execute(ctx, os.Args[1:]); err != nil {
		if errors.Is(err, context.Canceled) {
			fmt.Fprintln(os.Stderr, "operação cancelada")
			os.Exit(130)
		}

		fmt.Fprintf(os.Stderr, "erro: %v\n", err)
		os.Exit(1)
	}
}
