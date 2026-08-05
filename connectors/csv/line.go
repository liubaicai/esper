package csv

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/liubaicai/esper"
	"github.com/liubaicai/esper/connectors"
)

// LineSourceSpec configures the unformatted FileSourceLineUnformatted
// equivalent. Lines are emitted without their line terminator; blank lines
// are preserved. Exactly one of Path, Reader, or Open must be supplied.
type LineSourceSpec struct {
	Path            string
	Reader          io.Reader
	Open            func() (io.ReadCloser, error)
	CloseReader     bool
	Loop            bool
	EventsPerSecond int
	BufferSize      int
}

// LineSource emits raw text lines with the same lifecycle and replay contract
// as Source.
type LineSource struct {
	manager *connectors.StateManager
	spec    LineSourceSpec

	mu          sync.Mutex
	reader      *bufio.Reader
	closer      io.Closer
	seeker      io.Seeker
	readSeeker  io.ReadSeeker
	ownedCloser bool
	rowCount    int

	runMu sync.Mutex
}

func NewLineSource(spec LineSourceSpec) (*LineSource, error) {
	sources := 0
	if strings.TrimSpace(spec.Path) != "" {
		sources++
	}
	if spec.Reader != nil {
		sources++
	}
	if spec.Open != nil {
		sources++
	}
	if sources != 1 {
		return nil, fmt.Errorf("csv: exactly one of Path, Reader, or Open is required")
	}
	if spec.EventsPerSecond < 0 || spec.EventsPerSecond > 1000 {
		return nil, fmt.Errorf("csv: EventsPerSecond must be between 0 and 1000")
	}
	if spec.BufferSize < 0 {
		return nil, fmt.Errorf("csv: BufferSize cannot be negative")
	}
	return &LineSource{manager: connectors.NewStateManager(), spec: spec}, nil
}

func (s *LineSource) State() connectors.State {
	if s == nil {
		return connectors.Destroyed
	}
	return s.manager.State()
}

func (s *LineSource) Start() error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	if err := s.manager.Start(); err != nil {
		return err
	}
	s.mu.Lock()
	err := s.openLocked()
	s.mu.Unlock()
	if err != nil {
		s.mu.Lock()
		_ = s.closeLocked()
		s.mu.Unlock()
		_ = s.manager.Stop()
		return err
	}
	return nil
}

func (s *LineSource) Stop() error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	return s.manager.Stop()
}

func (s *LineSource) Pause() error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	return s.manager.Pause()
}

func (s *LineSource) Resume() error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	return s.manager.Resume()
}

func (s *LineSource) Destroy() error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	if err := s.manager.Destroy(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closeLocked()
}

func (s *LineSource) Reset() error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	if s.State() == connectors.Destroyed {
		return connectors.ErrDestroyed
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.reader == nil {
		return nil
	}
	if s.readSeeker != nil {
		if _, err := s.readSeeker.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("csv: reset line source: %w", err)
		}
		s.reader = newLineReader(s.readSeeker, s.spec.BufferSize)
		return nil
	}
	if strings.TrimSpace(s.spec.Path) != "" || s.spec.Open != nil {
		if err := s.closeLocked(); err != nil {
			return err
		}
		return s.openLocked()
	}
	return fmt.Errorf("csv: source is not resettable")
}

func (s *LineSource) RowCount() int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rowCount
}

func (s *LineSource) Next(ctx context.Context) (string, error) {
	if s == nil {
		return "", connectors.ErrDestroyed
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := s.manager.RequireStarted(); err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.reader == nil {
		if err := s.openLocked(); err != nil {
			return "", err
		}
	}
	line, err := s.reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("csv: read line: %w", err)
	}
	if err == io.EOF && line == "" {
		if s.spec.Loop {
			if resetErr := s.resetLocked(); resetErr != nil {
				return "", resetErr
			}
			line, err = s.reader.ReadString('\n')
			if err == io.EOF && line == "" {
				if s.manager.State() == connectors.Started {
					_ = s.manager.Stop()
				}
				return "", io.EOF
			}
		} else {
			if s.manager.State() == connectors.Started {
				_ = s.manager.Stop()
			}
			return "", io.EOF
		}
	}
	line = strings.TrimSuffix(line, "\n")
	line = strings.TrimSuffix(line, "\r")
	s.rowCount++
	return line, nil
}

func (s *LineSource) Run(ctx context.Context, emit func(context.Context, string) error) error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	if emit == nil {
		return fmt.Errorf("csv: Run requires a non-nil emit callback")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	s.runMu.Lock()
	defer s.runMu.Unlock()
	pacer := replayPacer{}
	for {
		if err := s.manager.WaitUntilStarted(ctx); err != nil {
			if errors.Is(err, connectors.ErrStopped) {
				return nil
			}
			return err
		}
		line, err := s.Next(ctx)
		if errors.Is(err, connectors.ErrPaused) || errors.Is(err, connectors.ErrNotStarted) {
			continue
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		due, err := pacer.nextDue(SourceSpec{EventsPerSecond: s.spec.EventsPerSecond}, nil, time.Now())
		if err != nil {
			return err
		}
		if err := waitForDue(ctx, s.manager, due); err != nil {
			if errors.Is(err, connectors.ErrStopped) {
				return nil
			}
			return err
		}
		if err := emit(ctx, line); err != nil {
			return err
		}
	}
}

func (s *LineSource) RunToEngine(ctx context.Context, engine *esper.Engine, eventType string) error {
	if engine == nil {
		return fmt.Errorf("csv: RunToEngine requires an engine")
	}
	if strings.TrimSpace(eventType) == "" {
		return fmt.Errorf("csv: RunToEngine requires an event type")
	}
	return s.Run(ctx, func(ctx context.Context, line string) error {
		return engine.Send(ctx, eventType, line)
	})
}

func (s *LineSource) openLocked() error {
	var (
		reader     io.Reader
		closer     io.Closer
		seeker     io.Seeker
		readSeeker io.ReadSeeker
		owned      bool
	)
	switch {
	case strings.TrimSpace(s.spec.Path) != "":
		file, err := os.Open(s.spec.Path)
		if err != nil {
			return fmt.Errorf("csv: open %q: %w", s.spec.Path, err)
		}
		reader, closer, seeker, owned = file, file, file, true
	case s.spec.Open != nil:
		opened, err := s.spec.Open()
		if err != nil {
			return fmt.Errorf("csv: open line source: %w", err)
		}
		if opened == nil {
			return fmt.Errorf("csv: Open returned a nil reader")
		}
		reader, closer, owned = opened, opened, true
		if value, ok := opened.(io.Seeker); ok {
			seeker = value
		}
		if value, ok := opened.(io.ReadSeeker); ok {
			readSeeker = value
		}
	default:
		reader = s.spec.Reader
		if s.spec.CloseReader {
			closer, owned = s.spec.Reader.(io.Closer)
		}
		if value, ok := s.spec.Reader.(io.Seeker); ok {
			seeker = value
		}
		if value, ok := s.spec.Reader.(io.ReadSeeker); ok {
			readSeeker = value
		}
	}
	s.reader = newLineReader(reader, s.spec.BufferSize)
	s.closer = closer
	s.seeker = seeker
	s.readSeeker = readSeeker
	s.ownedCloser = owned
	return nil
}

func newLineReader(reader io.Reader, bufferSize int) *bufio.Reader {
	if bufferSize > 0 {
		return bufio.NewReaderSize(reader, bufferSize)
	}
	return bufio.NewReader(reader)
}

func (s *LineSource) resetLocked() error {
	if s.readSeeker != nil {
		if _, err := s.readSeeker.Seek(0, io.SeekStart); err != nil {
			return err
		}
		s.reader = newLineReader(s.readSeeker, s.spec.BufferSize)
		return nil
	}
	if strings.TrimSpace(s.spec.Path) != "" || s.spec.Open != nil {
		if err := s.closeLocked(); err != nil {
			return err
		}
		return s.openLocked()
	}
	return fmt.Errorf("csv: source is not resettable")
}

func (s *LineSource) closeLocked() error {
	if s.closer == nil || !s.ownedCloser {
		s.reader = nil
		s.closer = nil
		s.seeker = nil
		s.readSeeker = nil
		return nil
	}
	err := s.closer.Close()
	s.reader = nil
	s.closer = nil
	s.seeker = nil
	s.readSeeker = nil
	s.ownedCloser = false
	return err
}

func waitForDue(ctx context.Context, manager *connectors.StateManager, due time.Time) error {
	for {
		if err := manager.WaitUntilStarted(ctx); err != nil {
			return err
		}
		delay := time.Until(due)
		if delay <= 0 {
			return nil
		}
		timer := time.NewTimer(delay)
		changed := manager.Changes()
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return ctx.Err()
		case <-changed:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			continue
		case <-timer.C:
			return nil
		}
	}
}
