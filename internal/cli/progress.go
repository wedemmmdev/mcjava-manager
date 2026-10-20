package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

const progressInterval = 150 * time.Millisecond

type progressPrinter struct {
	out        io.Writer
	enabled    bool
	started    bool
	last       time.Time
	lastLength int
}

func newProgressPrinter(out io.Writer) *progressPrinter {
	enabled := false
	if file, ok := out.(*os.File); ok {
		if info, err := file.Stat(); err == nil && info.Mode()&os.ModeCharDevice != 0 {
			enabled = true
		}
	}

	return &progressPrinter{out: out, enabled: enabled}
}

func (p *progressPrinter) Update(downloaded, total int64) error {
	if p == nil || !p.enabled {
		return nil
	}

	now := time.Now()
	if p.started && now.Sub(p.last) < progressInterval && (total <= 0 || downloaded < total) {
		return nil
	}
	p.started = true
	p.last = now

	var line string
	if total > 0 {
		percentage := float64(downloaded) * 100 / float64(total)
		if percentage > 100 {
			percentage = 100
		}
		line = fmt.Sprintf(
			"Baixando... %5.1f%% (%s/%s)",
			percentage,
			formatBytes(downloaded),
			formatBytes(total),
		)
	} else {
		line = fmt.Sprintf("Baixando... %s", formatBytes(downloaded))
	}

	p.lastLength = len(line)
	_, err := fmt.Fprintf(p.out, "\r%s", line)

	return err
}

func (p *progressPrinter) Finish(success bool) error {
	if p == nil || !p.enabled || !p.started {
		return nil
	}

	status := "concluído"
	if !success {
		status = "interrompido"
	}

	_, err := fmt.Fprintf(
		p.out,
		"\r%s\rBaixando... %s\n",
		strings.Repeat(" ", p.lastLength),
		status,
	)

	return err
}
