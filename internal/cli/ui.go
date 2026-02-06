package cli

import (
	"fmt"
	"io"
	"sync"
	"time"
)

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
)

type UI struct {
	out   io.Writer
	on    bool
	color bool
	mu    sync.Mutex
	spin  *spinner
}

func NewUI(out io.Writer, enabled, color bool) *UI {
	return &UI{out: out, on: enabled, color: color}
}

func (u *UI) Enabled() bool {
	return u != nil && u.on
}

func (u *UI) Step(message string, fn func() error) error {
	if u == nil || !u.on {
		return fn()
	}
	spinner := newSpinner(u, message, u.color)
	u.mu.Lock()
	u.spin = spinner
	u.mu.Unlock()
	spinner.Start()
	err := fn()
	spinner.Stop(err == nil)
	u.mu.Lock()
	u.spin = nil
	u.mu.Unlock()
	return err
}

func (u *UI) StepNoSpinner(message string, fn func() error) error {
	if u == nil || !u.on {
		return fn()
	}
	u.Infof("%s...", message)
	return fn()
}

func (u *UI) Infof(format string, args ...any) {
	u.printf(colorCyan, format, args...)
}

func (u *UI) Successf(format string, args ...any) {
	u.printf(colorGreen, format, args...)
}

func (u *UI) Warnf(format string, args ...any) {
	u.printf(colorYellow, format, args...)
}

func (u *UI) Errorf(format string, args ...any) {
	u.printf(colorRed, format, args...)
}

func (u *UI) printf(color, format string, args ...any) {
	if u == nil || !u.on {
		return
	}
	msg := fmt.Sprintf(format, args...)
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.spin != nil {
		fmt.Fprint(u.out, "\r\033[2K")
	}
	if u.color && color != "" {
		fmt.Fprintf(u.out, "%s%s%s\n", color, msg, colorReset)
		return
	}
	fmt.Fprintln(u.out, msg)
}

type spinner struct {
	ui    *UI
	out   io.Writer
	msg   string
	color bool
	stop  chan struct{}
}

func newSpinner(ui *UI, msg string, color bool) *spinner {
	return &spinner{ui: ui, out: ui.out, msg: msg, color: color, stop: make(chan struct{})}
}

func (s *spinner) Start() {
	frames := []string{"|", "/", "-", "\\"}
	go func() {
		i := 0
		for {
			select {
			case <-s.stop:
				return
			default:
				frame := frames[i%len(frames)]
				s.ui.mu.Lock()
				fmt.Fprint(s.out, "\r\033[2K")
				if s.color {
					fmt.Fprintf(s.out, "%s%s%s %s", colorYellow, frame, colorReset, s.msg)
				} else {
					fmt.Fprintf(s.out, "%s %s", frame, s.msg)
				}
				s.ui.mu.Unlock()
				i++
				time.Sleep(120 * time.Millisecond)
			}
		}
	}()
}

func (s *spinner) Stop(success bool) {
	close(s.stop)
	s.ui.mu.Lock()
	defer s.ui.mu.Unlock()
	fmt.Fprint(s.out, "\r\033[2K")
	status := "ok"
	color := colorGreen
	if !success {
		status = "err"
		color = colorRed
	}
	if s.color {
		fmt.Fprintf(s.out, "%s[%s]%s %s\n", color, status, colorReset, s.msg)
		return
	}
	fmt.Fprintf(s.out, "[%s] %s\n", status, s.msg)
}
