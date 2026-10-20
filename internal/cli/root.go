package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/wedemmmdev/mcjava-manager/internal/version"
)

// options contains the global flags shared by every command.
type options struct {
	InstancePath string
	ConfigPath   string
	Verbose      bool
	NoColor      bool
}

func (a *App) newRootCommand() *cobra.Command {
	opts := &options{}

	cmd := &cobra.Command{
		Use:     "mcjava-manager [instância]",
		Short:   "Gerencie mods, shaders, resourcepacks e mapas de instâncias Minecraft Java",
		Version: version.Short(),
		Long: `mcjava-manager gerencia mods, shaders, resourcepacks e mapas de instâncias
do Minecraft Java diretamente pelo terminal.

A instância pode ser informada como argumento posicional ou pela flag
persistente --instance.`,
		Example: `  mcjava-manager ./fabric-loader-0.19.5-1.20.1
  mcjava-manager --instance ./fabric-loader-0.19.5-1.20.1 version
  mcjava-manager search sodium --loader fabric --game-version 1.20.1
  mcjava-manager info sodium
  mcjava-manager versions sodium --game-version 1.20.1 --loader fabric
  mcjava-manager install sodium --instance ./fabric-loader-0.19.5-1.20.1 --game-version 1.20.1 --loader fabric
  mcjava-manager list --instance ./fabric-loader-0.19.5-1.20.1
  mcjava-manager update --instance ./fabric-loader-0.19.5-1.20.1 --check
  mcjava-manager remove sodium --instance ./fabric-loader-0.19.5-1.20.1
  mcjava-manager version`,
		Args:          cobra.MaximumNArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.resolve(args); err != nil {
				return err
			}

			if opts.InstancePath == "" {
				return cmd.Help()
			}

			return a.runTUI(cmd.Context(), opts)
		},
	}

	flags := cmd.PersistentFlags()
	flags.StringVarP(&opts.InstancePath, "instance", "i", "", "caminho da instância do Minecraft (env: MCJAVA_MANAGER_INSTANCE)")
	flags.StringVar(&opts.ConfigPath, "config", "", "arquivo de configuração (padrão: ~/.mcjava-manager.yaml, env: MCJAVA_MANAGER_CONFIG)")
	flags.BoolVarP(&opts.Verbose, "verbose", "v", false, "exibe logs detalhados")
	flags.BoolVar(&opts.NoColor, "no-color", false, "desativa cores e estilos visuais")

	cmd.SetVersionTemplate("mcjava-manager {{.Version}}\n")
	cmd.AddCommand(newVersionCommand())
	cmd.AddCommand(newSearchCommand(nil))
	cmd.AddCommand(newInfoCommand(nil))
	cmd.AddCommand(newVersionsCommand(nil))
	cmd.AddCommand(newInstallCommand(nil, opts, nil))
	cmd.AddCommand(newListCommand(opts))
	cmd.AddCommand(newRemoveCommand(opts))
	cmd.AddCommand(newUpdateCommand(nil, opts, nil))

	return cmd
}

// resolve applies flag precedence, expands a leading ~, and turns relative
// paths into absolute paths before any command touches the filesystem.
func (o *options) resolve(args []string) error {
	if len(args) > 0 {
		if o.InstancePath != "" && !samePath(o.InstancePath, args[0]) {
			return fmt.Errorf("a instância foi informada duas vezes (--instance e argumento posicional)")
		}
		o.InstancePath = args[0]
	}

	if o.ConfigPath == "" {
		o.ConfigPath = os.Getenv("MCJAVA_MANAGER_CONFIG")
	}

	var err error
	if o.InstancePath, err = o.instancePath(); err != nil {
		return err
	}
	if o.ConfigPath, err = absolutePath(o.ConfigPath); err != nil {
		return fmt.Errorf("falha ao resolver o caminho da configuração: %w", err)
	}

	return nil
}

func (o *options) instancePath() (string, error) {
	if o == nil {
		return "", fmt.Errorf("opções da CLI não inicializadas")
	}

	path := o.InstancePath
	if path == "" {
		path = os.Getenv("MCJAVA_MANAGER_INSTANCE")
	}

	absolute, err := absolutePath(path)
	if err != nil {
		return "", fmt.Errorf("falha ao resolver o caminho da instância: %w", err)
	}

	return absolute, nil
}

func (o *options) resolveInstance() (string, error) {
	path, err := o.instancePath()
	if err != nil {
		return "", err
	}
	if path == "" {
		return "", fmt.Errorf("informe o caminho da instância com --instance ou defina MCJAVA_MANAGER_INSTANCE")
	}

	return path, nil
}

func absolutePath(path string) (string, error) {
	if path == "" {
		return "", nil
	}

	expanded, err := expandHome(path)
	if err != nil {
		return "", err
	}

	absolute, err := filepath.Abs(expanded)
	if err != nil {
		return "", err
	}

	return filepath.Clean(absolute), nil
}

func expandHome(path string) (string, error) {
	if path != "~" && !strings.HasPrefix(path, "~/") && !strings.HasPrefix(path, `~\`) {
		return path, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("não foi possível localizar o diretório home: %w", err)
	}
	if path == "~" {
		return home, nil
	}

	return filepath.Join(home, path[2:]), nil
}

func samePath(a, b string) bool {
	aAbs, aErr := filepath.Abs(a)
	bAbs, bErr := filepath.Abs(b)
	if aErr != nil || bErr != nil {
		aAbs, bAbs = a, b
	}

	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(aAbs), filepath.Clean(bAbs))
	}

	return filepath.Clean(aAbs) == filepath.Clean(bAbs)
}
