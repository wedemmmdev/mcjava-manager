package instance

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"runtime"
	"strings"
)

// PermissionError describes a filesystem permission failure together with
// platform-specific instructions for fixing it.
type PermissionError struct {
	Operation string
	Path      string
	Err       error
}

func (e *PermissionError) Error() string {
	if e == nil {
		return "<nil>"
	}

	operation := e.Operation
	if operation == "" {
		operation = "acessar"
	}

	return fmt.Sprintf("permissão negada ao %s %q.\n\n%s", operation, e.Path, permissionGuidance(e.Path))
}

func (e *PermissionError) Unwrap() error {
	if e == nil {
		return nil
	}

	return e.Err
}

// IsPermissionError reports whether err or any wrapped error is a permission
// failure.
func IsPermissionError(err error) bool {
	return errors.Is(err, fs.ErrPermission)
}

func wrapFSError(operation, path string, err error) error {
	if errors.Is(err, fs.ErrPermission) {
		return &PermissionError{
			Operation: operation,
			Path:      path,
			Err:       err,
		}
	}

	return fmt.Errorf("falha ao %s %q: %w", operation, path, err)
}

func permissionGuidance(path string) string {
	switch {
	case runtime.GOOS == "windows":
		return "No Windows, execute o terminal como Administrador ou escolha uma pasta com permissão de escrita, como uma pasta dentro do seu perfil de usuário. Verifique também se a Proteção contra Ransomware ou o antivírus não estão bloqueando o diretório."
	case isTermux():
		return fmt.Sprintf("No Termux, conceda acesso ao armazenamento executando:\n\n  termux-setup-storage\n\nDepois use uma pasta acessível, como ~/storage/shared/..., e confirme a permissão de escrita em %q.", path)
	case runtime.GOOS == "android":
		return "No Android, conceda acesso ao armazenamento ao aplicativo e escolha uma pasta com permissão de escrita."
	case runtime.GOOS == "darwin":
		return fmt.Sprintf("No macOS, verifique Ajustes do Sistema > Privacidade e Segurança > Arquivos e Pastas (ou Acesso Total ao Disco) e garanta permissão de escrita em %q.", path)
	default:
		return fmt.Sprintf("Verifique as permissões do diretório %q e dos diretórios pais com:\n\n  ls -ld %q\n\nSe for seguro, conceda escrita ao seu usuário com:\n\n  chmod u+w %q", path, path, path)
	}
}

func isTermux() bool {
	return os.Getenv("TERMUX_VERSION") != "" || strings.Contains(os.Getenv("PREFIX"), "com.termux")
}
