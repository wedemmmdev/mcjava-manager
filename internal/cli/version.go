package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/wedemmm/mcjava-manager/internal/version"
)

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Exibe versão, commit e ambiente de compilação",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "mcjava-manager %s\n", version.Full()); err != nil {
				return fmt.Errorf("falha ao escrever a versão: %w", err)
			}

			return nil
		},
	}
}
