package internal

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/schollz/progressbar/v3"
)

const (
	ansiClearLine       = "\r\x1b[2K"
	ansiCursorUpOneLine = "\x1b[1A"
	historyBarWidth     = 20
)

var historySpinnerFrames = [...]string{"|", "/", "-", `\`}

// ModuleSpinner renders module scan progress as a terminal spinner.
type ModuleSpinner struct {
	writer    io.Writer
	bar       *progressbar.ProgressBar
	total     int
	completed int
	module    string
	startedAt time.Time
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
	s.module = ""
	s.startedAt = time.Now()
	s.stop = make(chan struct{})
	stop := s.stop
	s.bar = progressbar.NewOptions(-1,
		progressbar.OptionSetDescription(s.description(0)),
		progressbar.OptionSetWriter(s.writer),
		progressbar.OptionClearOnFinish(),
		progressbar.OptionSetElapsedTime(false),
		progressbar.OptionSpinnerType(14))
	s.mu.Unlock()

	s.wg.Add(1)
	go s.spin(stop)
}

// AdvanceModule reports that a module scan has finished.
func (s *ModuleSpinner) AdvanceModule(module string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.bar == nil {
		return
	}
	s.module = module
	if s.completed < s.total {
		s.completed++
	}
	s.bar.Describe(s.description(s.completed))
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
				s.bar.Describe(s.description(s.completed))
				_ = s.bar.Add(1)
			}
			s.mu.Unlock()
		case <-stop:
			return
		}
	}
}

func (s *ModuleSpinner) description(completed int) string {
	elapsed := time.Since(s.startedAt).Truncate(time.Second)
	if s.module != "" {
		return fmt.Sprintf("Scanning %d/%d modules [%s]: %s", completed, s.total, elapsed, s.module)
	}
	return fmt.Sprintf("Scanning %d/%d modules [%s]", completed, s.total, elapsed)
}

// HistoryProgressBar renders history progress and active module progress on separate terminal lines.
type HistoryProgressBar struct {
	writer    io.Writer
	total     int
	completed int
	timestamp time.Time
	startedAt time.Time
	module    historyModuleProgressState
	stop      chan struct{}
	wg        sync.WaitGroup
	mu        sync.Mutex
	started   bool
	rendered  bool
}

// NewHistoryProgressBar returns a progress renderer that writes history sample progress to writer.
func NewHistoryProgressBar(writer io.Writer) *HistoryProgressBar {
	return &HistoryProgressBar{writer: writer}
}

// NewModuleProgress returns module progress that renders below the history progress line.
func (p *HistoryProgressBar) NewModuleProgress() *HistoryModuleProgress {
	return &HistoryModuleProgress{parent: p}
}

// Start initializes the progress bar for total history samples.
func (p *HistoryProgressBar) Start(total int) {
	if total <= 0 {
		return
	}
	p.mu.Lock()
	p.total = total
	p.completed = 0
	p.timestamp = time.Time{}
	p.startedAt = time.Now()
	p.module = historyModuleProgressState{}
	p.stop = make(chan struct{})
	stop := p.stop
	p.started = true
	p.rendered = false
	p.renderLocked()
	p.wg.Add(1)
	p.mu.Unlock()

	go p.refresh(stop)
}

// StartSample reports that a historical sample has started.
func (p *HistoryProgressBar) StartSample(timestamp time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.started {
		return
	}
	p.timestamp = timestamp
	p.renderLocked()
}

// AdvanceSample reports that a historical sample has finished.
func (p *HistoryProgressBar) AdvanceSample(timestamp time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.started {
		return
	}
	p.timestamp = timestamp
	if p.completed < p.total {
		p.completed++
	}
	p.renderLocked()
}

// Finish clears the progress bar.
func (p *HistoryProgressBar) Finish() {
	p.mu.Lock()
	if !p.started {
		p.mu.Unlock()
		return
	}
	stop := p.stop
	p.stop = nil
	p.started = false
	p.mu.Unlock()

	if stop != nil {
		close(stop)
		p.wg.Wait()
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.clearLocked()
}

func (p *HistoryProgressBar) refresh(stop <-chan struct{}) {
	defer p.wg.Done()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			p.mu.Lock()
			if p.started {
				p.renderLocked()
			}
			p.mu.Unlock()
		case <-stop:
			return
		}
	}
}

func (p *HistoryProgressBar) renderLocked() {
	if !p.started {
		return
	}
	if p.rendered {
		p.advanceModuleFrameLocked()
		_, _ = fmt.Fprint(p.writer, ansiCursorUpOneLine)
	}
	_, _ = fmt.Fprint(p.writer, ansiClearLine, p.historyLine(), "\n", ansiClearLine, p.moduleLine())
	p.rendered = true
}

func (p *HistoryProgressBar) advanceModuleFrameLocked() {
	if p.module.active {
		p.module.frame++
	}
}

func (p *HistoryProgressBar) clearLocked() {
	if !p.rendered {
		return
	}
	_, _ = fmt.Fprint(p.writer, ansiCursorUpOneLine, ansiClearLine, "\n", ansiClearLine, ansiCursorUpOneLine, "\r")
	p.rendered = false
}

func (p *HistoryProgressBar) historyLine() string {
	return fmt.Sprintf(
		"%s %3d%% |%s| (%d/%d)",
		p.description(),
		p.historyPercent(),
		p.historyBar(),
		p.completed,
		p.total,
	)
}

func (p *HistoryProgressBar) description() string {
	elapsed := time.Since(p.startedAt).Truncate(time.Second)
	if !p.timestamp.IsZero() {
		return fmt.Sprintf(
			"Sampling %d/%d history [%s]: %s",
			p.completed, p.total, elapsed, p.timestamp.UTC().Format(time.DateOnly),
		)
	}
	return fmt.Sprintf("Sampling %d/%d history [%s]", p.completed, p.total, elapsed)
}

func (p *HistoryProgressBar) historyPercent() int {
	if p.total <= 0 {
		return 0
	}
	completed := min(max(p.completed, 0), p.total)
	return completed * 100 / p.total
}

func (p *HistoryProgressBar) historyBar() string {
	if p.total <= 0 {
		return strings.Repeat(" ", historyBarWidth)
	}
	completed := min(max(p.completed, 0), p.total)
	filled := historyBarWidth * completed / p.total
	return strings.Repeat("=", filled) + strings.Repeat(" ", historyBarWidth-filled)
}

func (p *HistoryProgressBar) moduleLine() string {
	if !p.module.active {
		return ""
	}
	elapsed := time.Since(p.module.startedAt).Truncate(time.Second)
	frame := historySpinnerFrames[p.module.frame%len(historySpinnerFrames)]
	if p.module.module != "" {
		return fmt.Sprintf(
			"%s Scanning %d/%d modules [%s]: %s",
			frame, p.module.completed, p.module.total, elapsed, p.module.module,
		)
	}
	return fmt.Sprintf("%s Scanning %d/%d modules [%s]", frame, p.module.completed, p.module.total, elapsed)
}

type historyModuleProgressState struct {
	total     int
	completed int
	module    string
	startedAt time.Time
	frame     int
	active    bool
}

// HistoryModuleProgress renders module scan progress below a history progress line.
type HistoryModuleProgress struct {
	parent *HistoryProgressBar
}

// Start initializes the module progress line for total modules.
func (p *HistoryModuleProgress) Start(total int) {
	if p == nil || p.parent == nil {
		return
	}
	if total <= 0 {
		return
	}
	p.parent.mu.Lock()
	defer p.parent.mu.Unlock()

	p.parent.module = historyModuleProgressState{
		total:     total,
		startedAt: time.Now(),
		active:    true,
	}
	p.parent.renderLocked()
}

// AdvanceModule reports that a module scan has finished.
func (p *HistoryModuleProgress) AdvanceModule(module string) {
	if p == nil || p.parent == nil {
		return
	}
	p.parent.mu.Lock()
	defer p.parent.mu.Unlock()

	if !p.parent.module.active {
		return
	}
	p.parent.module.module = module
	if p.parent.module.completed < p.parent.module.total {
		p.parent.module.completed++
	}
	p.parent.renderLocked()
}

// Advance reports that one module scan has finished.
func (p *HistoryModuleProgress) Advance() {
	if p == nil || p.parent == nil {
		return
	}
	p.parent.mu.Lock()
	defer p.parent.mu.Unlock()

	if !p.parent.module.active {
		return
	}
	if p.parent.module.completed < p.parent.module.total {
		p.parent.module.completed++
	}
	p.parent.renderLocked()
}

// Finish clears the module progress line.
func (p *HistoryModuleProgress) Finish() {
	if p == nil || p.parent == nil {
		return
	}
	p.parent.mu.Lock()
	defer p.parent.mu.Unlock()

	if !p.parent.module.active {
		return
	}
	p.parent.module = historyModuleProgressState{}
	p.parent.renderLocked()
}
