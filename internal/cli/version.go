package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/wedemmmdev/modink/internal/version"
)

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Exibe versão, commit e ambiente de compilação",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Modink %s\n", version.Full()); err != nil {
				return fmt.Errorf("falha ao escrever a versão: %w", err)
			}

			return nil
		},
	}
}
