package config

import "strings"

// Secret is a credential that redacts itself when formatted or serialized.
// Use Value only at the boundary that needs the raw credential.
type Secret string

// NewSecret normalizes and wraps a credential.
func NewSecret(value string) Secret {
	return Secret(strings.TrimSpace(value))
}

// Value returns the plaintext credential. Never log the returned string.
func (s Secret) Value() string {
	return string(s)
}

// IsSet reports whether the credential has a non-empty value.
func (s Secret) IsSet() bool {
	return strings.TrimSpace(string(s)) != ""
}

// String redacts the credential for user-facing output and logs.
func (s Secret) String() string {
	if !s.IsSet() {
		return ""
	}

	return "[REDACTED]"
}

// GoString redacts the credential when using %#v.
func (s Secret) GoString() string {
	return `config.Secret("[REDACTED]")`
}

// MarshalYAML prevents credentials from leaking into serialized config files.
// A future Save implementation must use a separate wire type if it needs to
// persist the real value.
func (s Secret) MarshalYAML() (any, error) {
	if !s.IsSet() {
		return "", nil
	}

	return "[REDACTED]", nil
}

// MarshalJSON prevents credentials from leaking through JSON serialization.
func (s Secret) MarshalJSON() ([]byte, error) {
	if !s.IsSet() {
		return []byte(`""`), nil
	}

	return []byte(`"[REDACTED]"`), nil
}

// MarshalText prevents credentials from leaking through text serialization.
func (s Secret) MarshalText() ([]byte, error) {
	if !s.IsSet() {
		return []byte{}, nil
	}

	return []byte("[REDACTED]"), nil
}
