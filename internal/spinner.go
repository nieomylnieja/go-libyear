package internal

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/schollz/progressbar/v3"
)

// ModuleSpinner renders module scan progress as a terminal spinner.
type ModuleSpinner struct {
	writer    io.Writer
	bar       *progressbar.ProgressBar
	total     int
	completed int
	stop      chan struct{}
	wg        sync.WaitGroup
	mu        sync.Mutex
}

// NewModuleSpinner returns a spinner that writes module scan progress to writer.
func NewModuleSpinner(writer io.Writer) *ModuleSpinner {
	return &ModuleSpinner{writer: writer}
}

// Start initializes the spinner for total modules.
func (s *ModuleSpinner) Start(total int) {
	if total <= 0 {
		return
	}
	s.mu.Lock()
	s.total = total
	s.completed = 0
	s.stop = make(chan struct{})
	stop := s.stop
	s.bar = progressbar.NewOptions(-1,
		progressbar.OptionSetDescription(s.description(0)),
		progressbar.OptionSetWriter(s.writer),
		progressbar.OptionOnCompletion(func() { _, _ = fmt.Fprint(s.writer, "\n") }),
		progressbar.OptionClearOnFinish(),
		progressbar.OptionSpinnerType(14))
	s.mu.Unlock()

	s.wg.Add(1)
	go s.spin(stop)
}

// Advance reports that one module scan has finished.
func (s *ModuleSpinner) Advance() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.bar == nil {
		return
	}
	if s.completed < s.total {
		s.completed++
	}
	s.bar.Describe(s.description(s.completed))
}

// Finish stops the spinner at its current progress.
func (s *ModuleSpinner) Finish() {
	s.mu.Lock()
	if s.bar == nil {
		s.mu.Unlock()
		return
	}
	stop := s.stop
	s.stop = nil
	s.mu.Unlock()

	if stop != nil {
		close(stop)
		s.wg.Wait()
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.bar.Describe(s.description(s.completed))
	_ = s.bar.Finish()
}

func (s *ModuleSpinner) spin(stop <-chan struct{}) {
	defer s.wg.Done()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.mu.Lock()
			if s.bar != nil {
				_ = s.bar.Add(1)
			}
			s.mu.Unlock()
		case <-stop:
			return
		}
	}
}

func (s *ModuleSpinner) description(completed int) string {
	return fmt.Sprintf("Scanning %d/%d modules", completed, s.total)
}
