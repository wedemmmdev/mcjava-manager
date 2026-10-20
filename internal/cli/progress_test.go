package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestProgressPrinterDisabledForBuffer(t *testing.T) {
	var out bytes.Buffer
	printer := newProgressPrinter(&out)

	if printer.enabled {
		t.Fatal("buffer should not enable terminal progress")
	}
	if err := printer.Update(10, 100); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if err := printer.Finish(true); err != nil {
		t.Fatalf("Finish() error = %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("output = %q, want empty", out.String())
	}
}

func TestProgressPrinterFormatsPercentage(t *testing.T) {
	var out bytes.Buffer
	printer := &progressPrinter{out: &out, enabled: true}

	if err := printer.Update(0, 1024); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if !strings.Contains(out.String(), "0.0%") || !strings.Contains(out.String(), "1.0 KB") {
		t.Fatalf("output = %q, missing initial progress", out.String())
	}

	printer.last = time.Now().Add(-time.Second)
	if err := printer.Update(512, 1024); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if !strings.Contains(out.String(), "50.0%") || !strings.Contains(out.String(), "512 B") {
		t.Fatalf("output = %q, missing half progress", out.String())
	}

	if err := printer.Update(1024, 1024); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if !strings.Contains(out.String(), "100.0%") {
		t.Fatalf("output = %q, missing complete progress", out.String())
	}

	if err := printer.Finish(true); err != nil {
		t.Fatalf("Finish() error = %v", err)
	}
	if !strings.Contains(out.String(), "concluído") || !strings.HasSuffix(out.String(), "\n") {
		t.Fatalf("output = %q, missing completion line", out.String())
	}
}

func TestProgressPrinterWithoutTotalSize(t *testing.T) {
	var out bytes.Buffer
	printer := &progressPrinter{out: &out, enabled: true}

	if err := printer.Update(1024, 0); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if !strings.Contains(out.String(), "1.0 KB") {
		t.Fatalf("output = %q, missing downloaded bytes", out.String())
	}

	if err := printer.Finish(false); err != nil {
		t.Fatalf("Finish() error = %v", err)
	}
	if !strings.Contains(out.String(), "interrompido") {
		t.Fatalf("output = %q, missing interrupted status", out.String())
	}
}

func TestProgressPrinterThrottlesUpdates(t *testing.T) {
	var out bytes.Buffer
	printer := &progressPrinter{out: &out, enabled: true}

	if err := printer.Update(0, 100); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	before := out.Len()
	printer.last = time.Now().Add(time.Hour)
	if err := printer.Update(50, 100); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if out.Len() != before {
		t.Fatalf("output grew despite throttling: %q", out.String())
	}

	printer.last = time.Now().Add(-time.Second)
	if err := printer.Update(50, 100); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if out.Len() == before {
		t.Fatal("output did not grow after the throttle interval")
	}
}

type failingWriter struct {
	err error
}

func (w failingWriter) Write([]byte) (int, error) {
	return 0, w.err
}

func TestProgressPrinterPropagatesWriteErrors(t *testing.T) {
	writeErr := errors.New("stderr fechado")
	printer := &progressPrinter{out: failingWriter{err: writeErr}, enabled: true}

	if err := printer.Update(10, 100); !errors.Is(err, writeErr) {
		t.Fatalf("Update() error = %v, want %v", err, writeErr)
	}
	if err := printer.Finish(true); !errors.Is(err, writeErr) {
		t.Fatalf("Finish() error = %v, want %v", err, writeErr)
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		size int64
		want string
	}{
		{size: -1, want: "0 B"},
		{size: 0, want: "0 B"},
		{size: 1023, want: "1023 B"},
		{size: 1024, want: "1.0 KB"},
		{size: 1536, want: "1.5 KB"},
		{size: 1024 * 1024, want: "1.0 MB"},
		{size: 1024 * 1024 * 1024, want: "1.0 GB"},
	}

	for _, test := range tests {
		if got := formatBytes(test.size); got != test.want {
			t.Errorf("formatBytes(%d) = %q, want %q", test.size, got, test.want)
		}
	}
}
