package csv

import (
	"context"
	encodingcsv "encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/liubaicai/esper/connectors"
)

// SinkSpec describes a CSV sink. Exactly one of Path, Writer, or Open must be
// supplied. Columns are written in the supplied order; when omitted, the
// first record's keys are sorted and become the stable output schema.
type SinkSpec struct {
	Path        string
	Writer      io.Writer
	Open        func() (io.WriteCloser, error)
	CloseWriter bool

	Columns        []string
	Header         bool
	Append         bool
	FlushEachWrite bool
	Comma          rune
}

// Sink is a lifecycle-managed CSV writer. It accepts Record values and common
// Esper Event/Row/TableRow values through WriteEvent.
type Sink struct {
	manager *connectors.StateManager
	spec    SinkSpec

	mu            sync.Mutex
	writer        io.Writer
	csvWriter     *encodingcsv.Writer
	closer        io.Closer
	ownedCloser   bool
	columns       []string
	headerWritten bool

	writeCount int
}

func NewSink(spec SinkSpec) (*Sink, error) {
	if err := validateSinkSpec(spec); err != nil {
		return nil, err
	}
	if err := validateDelimiter(spec.Comma, 0); err != nil {
		return nil, err
	}
	spec.Columns = append([]string(nil), spec.Columns...)
	return &Sink{
		manager: connectors.NewStateManager(),
		spec:    spec,
		columns: append([]string(nil), spec.Columns...),
	}, nil
}

func validateSinkSpec(spec SinkSpec) error {
	sinks := 0
	if strings.TrimSpace(spec.Path) != "" {
		sinks++
	}
	if spec.Writer != nil {
		sinks++
	}
	if spec.Open != nil {
		sinks++
	}
	if sinks != 1 {
		return fmt.Errorf("csv: exactly one of Path, Writer, or Open is required")
	}
	seen := make(map[string]struct{}, len(spec.Columns))
	for _, name := range spec.Columns {
		name = strings.TrimSpace(name)
		if name == "" {
			return fmt.Errorf("csv: Columns contains an empty name")
		}
		if _, exists := seen[name]; exists {
			return fmt.Errorf("csv: Columns contains duplicate column %q", name)
		}
		seen[name] = struct{}{}
	}
	return nil
}

func (s *Sink) State() connectors.State {
	if s == nil {
		return connectors.Destroyed
	}
	return s.manager.State()
}

func (s *Sink) Start() error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	if err := s.manager.Start(); err != nil {
		return err
	}
	s.mu.Lock()
	err := s.openLocked()
	if err == nil && s.spec.Header && len(s.columns) > 0 && !s.headerWritten {
		err = s.writeHeaderLocked()
	}
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

func (s *Sink) Stop() error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	if err := s.manager.Stop(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closeLocked()
}

func (s *Sink) Pause() error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	return s.manager.Pause()
}

func (s *Sink) Resume() error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	return s.manager.Resume()
}

func (s *Sink) Destroy() error {
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

// Write emits one record. Unknown keys are ignored once a column list has
// been established, matching a fixed Esper event type's writable properties.
func (s *Sink) Write(ctx context.Context, record Record) error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.manager.RequireStarted(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.csvWriter == nil {
		return connectors.ErrNotStarted
	}
	if len(s.columns) == 0 {
		s.columns = sortedRecordKeys(record)
		if len(s.columns) == 0 {
			return fmt.Errorf("csv: cannot derive columns from an empty record")
		}
		if s.spec.Header && !s.headerWritten {
			if err := s.writeHeaderLocked(); err != nil {
				return err
			}
		}
	}
	values := make([]string, len(s.columns))
	for index, name := range s.columns {
		values[index] = formatCSVValue(record[name])
	}
	if err := s.csvWriter.Write(values); err != nil {
		return fmt.Errorf("csv: write record: %w", err)
	}
	if s.spec.FlushEachWrite {
		s.csvWriter.Flush()
		if err := s.csvWriter.Error(); err != nil {
			return fmt.Errorf("csv: flush record: %w", err)
		}
	}
	s.writeCount++
	return nil
}

// WriteEvent converts an Esper event/result or a Go struct to a Record and
// writes it using the same fixed-column contract as Write.
func (s *Sink) WriteEvent(ctx context.Context, value any) error {
	record, err := EventToRecord(value)
	if err != nil {
		return err
	}
	return s.Write(ctx, record)
}

func (s *Sink) WriteCount() int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeCount
}

func (s *Sink) openLocked() error {
	var (
		writer io.Writer
		closer io.Closer
		owned  bool
	)
	switch {
	case strings.TrimSpace(s.spec.Path) != "":
		flags := os.O_CREATE | os.O_WRONLY
		if s.spec.Append {
			flags |= os.O_APPEND
		} else {
			flags |= os.O_TRUNC
		}
		file, err := os.OpenFile(s.spec.Path, flags, 0o644)
		if err != nil {
			return fmt.Errorf("csv: open %q for writing: %w", s.spec.Path, err)
		}
		writer, closer, owned = file, file, true
		if s.spec.Append {
			if info, err := file.Stat(); err == nil && info.Size() > 0 {
				s.headerWritten = true
			}
		}
	case s.spec.Open != nil:
		opened, err := s.spec.Open()
		if err != nil {
			return fmt.Errorf("csv: open sink: %w", err)
		}
		if opened == nil {
			return fmt.Errorf("csv: Open returned a nil writer")
		}
		writer, closer, owned = opened, opened, true
	default:
		writer = s.spec.Writer
		if s.spec.CloseWriter {
			closer, owned = s.spec.Writer.(io.Closer)
		}
	}
	s.writer = writer
	s.closer = closer
	s.ownedCloser = owned
	s.csvWriter = encodingcsv.NewWriter(writer)
	if s.spec.Comma != 0 {
		s.csvWriter.Comma = s.spec.Comma
	}
	return nil
}

func (s *Sink) writeHeaderLocked() error {
	if len(s.columns) == 0 || s.csvWriter == nil {
		return nil
	}
	if err := s.csvWriter.Write(append([]string(nil), s.columns...)); err != nil {
		return fmt.Errorf("csv: write header: %w", err)
	}
	s.csvWriter.Flush()
	if err := s.csvWriter.Error(); err != nil {
		return fmt.Errorf("csv: flush header: %w", err)
	}
	s.headerWritten = true
	return nil
}

func (s *Sink) closeLocked() error {
	var firstErr error
	if s.csvWriter != nil {
		s.csvWriter.Flush()
		firstErr = s.csvWriter.Error()
	}
	if s.closer != nil && s.ownedCloser {
		if err := s.closer.Close(); firstErr == nil {
			firstErr = err
		}
	}
	s.writer = nil
	s.csvWriter = nil
	s.closer = nil
	s.ownedCloser = false
	if firstErr != nil {
		return fmt.Errorf("csv: close sink: %w", firstErr)
	}
	return nil
}

func sortedRecordKeys(record Record) []string {
	keys := make([]string, 0, len(record))
	for name := range record {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	return keys
}

func formatCSVValue(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case time.Time:
		return typed.Format(time.RFC3339Nano)
	case []byte:
		return string(typed)
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	case map[string]any, []any:
		if encoded, err := json.Marshal(value); err == nil {
			return string(encoded)
		}
	}
	return fmt.Sprint(value)
}

func structToRecord(value any) (Record, error) {
	if value == nil {
		return nil, fmt.Errorf("csv: cannot convert nil to a record")
	}
	reflected := reflect.ValueOf(value)
	for reflected.Kind() == reflect.Pointer {
		if reflected.IsNil() {
			return nil, fmt.Errorf("csv: cannot convert a nil pointer to a record")
		}
		reflected = reflected.Elem()
	}
	if reflected.Kind() != reflect.Struct {
		return nil, fmt.Errorf("csv: unsupported event value %T", value)
	}
	record := make(Record)
	if err := collectStructFields(reflected, record); err != nil {
		return nil, err
	}
	return record, nil
}

func collectStructFields(value reflect.Value, record Record) error {
	typ := value.Type()
	for index := 0; index < typ.NumField(); index++ {
		field := typ.Field(index)
		if field.PkgPath != "" && !field.Anonymous {
			continue
		}
		tag := field.Tag.Get("esper")
		if tag == "" {
			tag = field.Tag.Get("json")
		}
		name := parseStructTag(tag)
		if name == "-" {
			continue
		}
		fieldValue := value.Field(index)
		base := fieldValue
		for base.Kind() == reflect.Pointer {
			if base.IsNil() {
				break
			}
			base = base.Elem()
		}
		if field.Anonymous && name == "" && base.IsValid() && base.Kind() == reflect.Struct && base.Type() != reflect.TypeOf(time.Time{}) {
			if err := collectStructFields(base, record); err != nil {
				return err
			}
			continue
		}
		if name == "" {
			if field.PkgPath != "" {
				continue
			}
			name = field.Name
		}
		if !fieldValue.CanInterface() {
			continue
		}
		record[name] = fieldValue.Interface()
	}
	return nil
}

func parseStructTag(tag string) string {
	if tag == "" {
		return ""
	}
	parts := strings.Split(tag, ",")
	name := parts[0]
	if name == "" {
		name = ""
	}
	return name
}
