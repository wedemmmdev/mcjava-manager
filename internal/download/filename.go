package download

import (
	"fmt"
	"strings"
)

const maxFilenameBytes = 255

var windowsReservedNames = map[string]struct{}{
	"CON":  {},
	"PRN":  {},
	"AUX":  {},
	"NUL":  {},
	"COM1": {},
	"COM2": {},
	"COM3": {},
	"COM4": {},
	"COM5": {},
	"COM6": {},
	"COM7": {},
	"COM8": {},
	"COM9": {},
	"LPT1": {},
	"LPT2": {},
	"LPT3": {},
	"LPT4": {},
	"LPT5": {},
	"LPT6": {},
	"LPT7": {},
	"LPT8": {},
	"LPT9": {},
}

// SafeFilename validates a provider-supplied filename and returns it
// unchanged. Path separators and names invalid on Windows are rejected so the
// same download works on every supported platform.
func SafeFilename(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("%w: nome vazio", ErrInvalidFilename)
	}
	if name != strings.TrimSpace(name) {
		return "", fmt.Errorf("%w: %q tem espaços nas extremidades", ErrInvalidFilename, name)
	}
	if name == "." || name == ".." {
		return "", fmt.Errorf("%w: %q é um nome reservado", ErrInvalidFilename, name)
	}
	if strings.ContainsAny(name, `/\`) {
		return "", fmt.Errorf("%w: %q contém separador de caminho", ErrInvalidFilename, name)
	}
	if strings.ContainsAny(name, `<>:"|?*`) {
		return "", fmt.Errorf("%w: %q contém caractere inválido", ErrInvalidFilename, name)
	}
	if strings.HasSuffix(name, ".") || strings.HasSuffix(name, " ") {
		return "", fmt.Errorf("%w: %q termina com ponto ou espaço", ErrInvalidFilename, name)
	}
	if len(name) > maxFilenameBytes {
		return "", fmt.Errorf("%w: nome com mais de %d bytes", ErrInvalidFilename, maxFilenameBytes)
	}

	for _, r := range name {
		if r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f) ||
			(r >= 0x202a && r <= 0x202e) || (r >= 0x2066 && r <= 0x2069) {
			return "", fmt.Errorf("%w: %q contém caractere de controle", ErrInvalidFilename, name)
		}
	}

	stem := name
	if dot := strings.IndexRune(stem, '.'); dot >= 0 {
		stem = stem[:dot]
	}
	if _, reserved := windowsReservedNames[strings.ToUpper(stem)]; reserved {
		return "", fmt.Errorf("%w: %q é um nome reservado no Windows", ErrInvalidFilename, name)
	}

	return name, nil
}
