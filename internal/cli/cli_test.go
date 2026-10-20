package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppRunsVersionCommand(t *testing.T) {
	t.Setenv("MODINK_INSTANCE", "")
	t.Setenv("MODINK_CONFIG", "")

	var out, errOut bytes.Buffer
	app := NewApp(strings.NewReader(""), &out, &errOut)

	if err := app.Execute(context.Background(), []string{"version"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out.String(), "Modink") {
		t.Fatalf("version output = %q, want it to contain Modink", out.String())
	}
	if errOut.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", errOut.String())
	}
}

func TestAppShowsHelpWithoutInstance(t *testing.T) {
	t.Setenv("MODINK_INSTANCE", "")
	t.Setenv("MODINK_CONFIG", "")

	var out, errOut bytes.Buffer
	app := NewApp(strings.NewReader(""), &out, &errOut)

	if err := app.Execute(context.Background(), nil); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out.String(), "Usage:") {
		t.Fatalf("help output = %q, want usage information", out.String())
	}
	for _, command := range []string{"search", "info", "versions", "install"} {
		if !strings.Contains(out.String(), command) {
			t.Fatalf("help output = %q, want the %s command", out.String(), command)
		}
	}
}

func TestAppVersionFlag(t *testing.T) {
	var out, errOut bytes.Buffer
	app := NewApp(strings.NewReader(""), &out, &errOut)

	if err := app.Execute(context.Background(), []string{"--version"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out.String(), "Modink") {
		t.Fatalf("--version output = %q", out.String())
	}
}

func TestOptionsResolvePositionalInstance(t *testing.T) {
	t.Setenv("MODINK_INSTANCE", "")
	t.Setenv("MODINK_CONFIG", "")

	opts := &options{}
	if err := opts.resolve([]string{"./minha-instancia"}); err != nil {
		t.Fatalf("resolve() error = %v", err)
	}
	if !filepath.IsAbs(opts.InstancePath) {
		t.Fatalf("InstancePath = %q, want an absolute path", opts.InstancePath)
	}
	if filepath.Base(opts.InstancePath) != "minha-instancia" {
		t.Fatalf("InstancePath = %q, want base minha-instancia", opts.InstancePath)
	}
}

func TestOptionsResolveRejectsConflictingInstances(t *testing.T) {
	t.Setenv("MODINK_INSTANCE", "")
	t.Setenv("MODINK_CONFIG", "")

	opts := &options{InstancePath: "./primeira"}
	if err := opts.resolve([]string{"./segunda"}); err == nil {
		t.Fatal("resolve() error = nil, want a conflict error")
	}
}

func TestOptionsResolveAcceptsRepeatedInstance(t *testing.T) {
	t.Setenv("MODINK_INSTANCE", "")
	t.Setenv("MODINK_CONFIG", "")

	opts := &options{InstancePath: "./mesma"}
	if err := opts.resolve([]string{"./mesma"}); err != nil {
		t.Fatalf("resolve() error = %v, want nil", err)
	}
}

func TestOptionsResolveDoesNotValidateLoaderNames(t *testing.T) {
	t.Setenv("MODINK_INSTANCE", "")
	t.Setenv("MODINK_CONFIG", "")

	names := []string{
		"minha-instancia",
		"qualquer pasta 1.20.1",
		"fabric-nao-e-requisito",
		"servidor-de-amigos",
	}

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			opts := &options{}
			if err := opts.resolve([]string{name}); err != nil {
				t.Fatalf("resolve() error = %v, want nil for an arbitrary directory name", err)
			}
			if filepath.Base(opts.InstancePath) != name {
				t.Fatalf("InstancePath = %q, want base %q", opts.InstancePath, name)
			}
		})
	}
}

func TestOptionsResolveEnvironment(t *testing.T) {
	t.Setenv("MODINK_INSTANCE", "./instancia-do-ambiente")
	t.Setenv("MODINK_CONFIG", "./config-do-ambiente.yaml")

	opts := &options{}
	if err := opts.resolve(nil); err != nil {
		t.Fatalf("resolve() error = %v", err)
	}
	if filepath.Base(opts.InstancePath) != "instancia-do-ambiente" {
		t.Fatalf("InstancePath = %q", opts.InstancePath)
	}
	if filepath.Base(opts.ConfigPath) != "config-do-ambiente.yaml" {
		t.Fatalf("ConfigPath = %q", opts.ConfigPath)
	}
}

func TestOptionsResolveFlagOverridesEnvironment(t *testing.T) {
	t.Setenv("MODINK_INSTANCE", "./ambiente")
	t.Setenv("MODINK_CONFIG", "")

	opts := &options{InstancePath: "./flag"}
	if err := opts.resolve(nil); err != nil {
		t.Fatalf("resolve() error = %v", err)
	}
	if filepath.Base(opts.InstancePath) != "flag" {
		t.Fatalf("InstancePath = %q, want the flag value", opts.InstancePath)
	}
}

func TestAbsolutePathEmpty(t *testing.T) {
	got, err := absolutePath("")
	if err != nil {
		t.Fatalf("absolutePath() error = %v", err)
	}
	if got != "" {
		t.Fatalf("absolutePath(\"\") = %q, want empty", got)
	}
}

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir() error = %v", err)
	}

	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "home", path: "~", want: home},
		{name: "forward slash", path: "~/mods", want: filepath.Join(home, "mods")},
		{name: "no tilde", path: "mods", want: "mods"},
		{name: "other user", path: "~outro/mods", want: "~outro/mods"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := expandHome(tt.path)
			if err != nil {
				t.Fatalf("expandHome() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("expandHome(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestOptionsResolveInstanceUsesFlag(t *testing.T) {
	t.Setenv("MODINK_INSTANCE", "./ambiente")

	opts := &options{InstancePath: "./flag"}
	got, err := opts.resolveInstance()
	if err != nil {
		t.Fatalf("resolveInstance() error = %v", err)
	}
	if filepath.Base(got) != "flag" {
		t.Fatalf("resolveInstance() = %q, want flag path", got)
	}
}

func TestOptionsResolveInstanceUsesEnvironment(t *testing.T) {
	t.Setenv("MODINK_INSTANCE", "./ambiente")

	got, err := (&options{}).resolveInstance()
	if err != nil {
		t.Fatalf("resolveInstance() error = %v", err)
	}
	if filepath.Base(got) != "ambiente" {
		t.Fatalf("resolveInstance() = %q, want environment path", got)
	}
}

func TestOptionsResolveInstanceRequiresPath(t *testing.T) {
	t.Setenv("MODINK_INSTANCE", "")

	_, err := (&options{}).resolveInstance()
	if err == nil || !strings.Contains(err.Error(), "informe o caminho da instância") {
		t.Fatalf("resolveInstance() error = %v, want missing path error", err)
	}
}

func TestOptionsResolveInstanceNilOptions(t *testing.T) {
	if _, err := (*options)(nil).resolveInstance(); err == nil {
		t.Fatal("resolveInstance() on nil options should fail")
	}
}
