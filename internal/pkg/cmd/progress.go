package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// RunProgress runs a shell command and renders a lightweight spinner + elapsed time while it runs.
// It is intended for long-running install/update steps where silence looks like a hang.
//
// Behavior:
// - On TTY: prints a single updating line and then a success/failure line.
// - On non-TTY: prints start + end lines (no spinner).
// - Captures only a limited tail of output; on error, prints that tail.
func RunProgress(label string, command string) error {
	start := time.Now()

	cmd := exec.Command("bash", "-c", command)
	out := newLimitedBuffer(64 * 1024) // keep last 64KB
	cmd.Stdout = out
	cmd.Stderr = out

	tty := isTTY(os.Stdout)

	var spinnerDone chan struct{}
	if tty {
		spinnerDone = make(chan struct{})
		go renderSpinner(os.Stdout, spinnerDone, label, start)
	} else {
		fmt.Fprintf(os.Stdout, "- %s...\n", label)
	}

	err := cmd.Run()

	if tty {
		close(spinnerDone)
		clearLine(os.Stdout)
	}

	elapsed := time.Since(start).Round(100 * time.Millisecond)
	if err == nil {
		fmt.Fprintf(os.Stdout, "✓ %s (%s)\n", label, elapsed)
		return nil
	}

	fmt.Fprintf(os.Stdout, "✗ %s (%s)\n", label, elapsed)
	tail := strings.TrimSpace(out.String())
	if tail != "" {
		fmt.Fprintln(os.Stdout, "---- command output (tail) ----")
		fmt.Fprintln(os.Stdout, tail)
		fmt.Fprintln(os.Stdout, "------------------------------")
	}
	return err
}

func renderSpinner(w *os.File, done <-chan struct{}, label string, start time.Time) {
	frames := []rune{'|', '/', '-', '\\'}
	i := 0
	ticker := time.NewTicker(120 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			elapsed := time.Since(start).Round(time.Second)
			// \r + clear line keeps the UI from scrolling.
			fmt.Fprintf(w, "\r\x1b[2K%s... %c %s", label, frames[i%len(frames)], elapsed)
			i++
		}
	}
}

func clearLine(w *os.File) {
	fmt.Fprint(w, "\r\x1b[2K")
}

func isTTY(f *os.File) bool {
	st, err := f.Stat()
	if err != nil {
		return false
	}
	return (st.Mode() & os.ModeCharDevice) != 0
}

type limitedBuffer struct {
	mu    sync.Mutex
	limit int
	buf   bytes.Buffer
}

func newLimitedBuffer(limit int) *limitedBuffer {
	return &limitedBuffer{limit: limit}
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// If the incoming chunk alone exceeds the limit, keep only its tail.
	if len(p) >= b.limit {
		b.buf.Reset()
		_, _ = b.buf.Write(p[len(p)-b.limit:])
		return len(p), nil
	}

	// Ensure the buffer doesn't exceed the limit.
	if b.buf.Len()+len(p) > b.limit {
		overflow := b.buf.Len() + len(p) - b.limit
		existing := b.buf.Bytes()
		if overflow >= len(existing) {
			b.buf.Reset()
		} else {
			b.buf.Reset()
			_, _ = b.buf.Write(existing[overflow:])
		}
	}

	return b.buf.Write(p)
}

func (b *limitedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
