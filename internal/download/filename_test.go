package download

import (
	"errors"
	"strings"
	"testing"
)

func TestSafeFilename(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "jar válido", input: "sodium-fabric-0.8.13+mc1.21.1.jar", want: "sodium-fabric-0.8.13+mc1.21.1.jar"},
		{name: "zip válido", input: "shader.zip", want: "shader.zip"},
		{name: "unicode", input: "pacote-ção.zip", want: "pacote-ção.zip"},
		{name: "vazio", input: "", wantErr: true},
		{name: "ponto", input: ".", wantErr: true},
		{name: "dois pontos", input: "..", wantErr: true},
		{name: "barra", input: "../evil.jar", wantErr: true},
		{name: "barra invertida", input: `..\evil.jar`, wantErr: true},
		{name: "subdiretório", input: "mods/evil.jar", wantErr: true},
		{name: "espaço inicial", input: " evil.jar", wantErr: true},
		{name: "espaço final", input: "evil.jar ", wantErr: true},
		{name: "ponto final", input: "evil.jar.", wantErr: true},
		{name: "dois pontos windows", input: "evil:name.jar", wantErr: true},
		{name: "curinga", input: "evil*.jar", wantErr: true},
		{name: "controle", input: "evil\x1b.jar", wantErr: true},
		{name: "controle c1", input: "evil\u0085.jar", wantErr: true},
		{name: "bidi", input: "evil\u202e.jar", wantErr: true},
		{name: "marca bidi", input: "evil\u200e.jar", wantErr: true},
		{name: "marca árabe", input: "evil\u061c.jar", wantErr: true},
		{name: "separador de linha", input: "evil\u2028.jar", wantErr: true},
		{name: "reservado CON", input: "CON", wantErr: true},
		{name: "reservado con.txt", input: "con.txt", wantErr: true},
		{name: "reservado LPT1", input: "LPT1.jar", wantErr: true},
		{name: "nome longo", input: strings.Repeat("a", 256), wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := SafeFilename(test.input)
			if test.wantErr {
				if err == nil {
					t.Fatalf("SafeFilename(%q) error = nil, want an error", test.input)
				}
				if !errors.Is(err, ErrInvalidFilename) {
					t.Fatalf("SafeFilename(%q) error = %v, want ErrInvalidFilename", test.input, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("SafeFilename(%q) error = %v", test.input, err)
			}
			if got != test.want {
				t.Fatalf("SafeFilename(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}
