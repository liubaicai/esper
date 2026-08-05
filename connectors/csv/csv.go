// Package csv implements the first Go EsperIO connector slice: CSV input
// adapters and file-oriented replay. It intentionally exposes records as
// map-backed Go values so a caller can route them into a registered Esper map,
// JSON, or Avro schema without introducing an EPL parser.
package csv

import (
	"context"
	encodingcsv "encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	esper "github.com/liubaicai/esper"
	"github.com/liubaicai/esper/connectors"
)

// Record is the map-backed event emitted by Source and accepted by Sink.
type Record map[string]any

// ColumnType selects one of the built-in Java BasicTypeCoercer-compatible
// conversions. Columns default to ColumnTypeString.
type ColumnType uint8

const (
	ColumnTypeString ColumnType = iota
	ColumnTypeInt
	ColumnTypeInt64
	ColumnTypeFloat64
	ColumnTypeBool
	ColumnTypeTime
	ColumnTypeJSON
)

// Converter lets callers provide a Go conversion for a column type not
// covered by the built-ins. It is called after empty-value handling.
type Converter func(string) (any, error)

// SourceSpec describes a CSV input source. Exactly one of Path, Reader, or
// Open must be supplied. Reader is caller-owned unless CloseReader is true.
//
// HasTitleLine is the explicit FileSourceCSV/CSVInputAdapter title-row mode.
// When it is false and PropertyOrder is supplied, AutoDetectTitle recognizes a
// first row whose values are exactly the property names, matching Esper's CSV
// adapter behavior. PropertyOrder is required for a source without a title
// row; a title row can provide the property names itself.
type SourceSpec struct {
	Path        string
	Reader      io.Reader
	Open        func() (io.ReadCloser, error)
	CloseReader bool

	HasTitleLine    bool
	AutoDetectTitle bool
	PropertyOrder   []string
	PropertyTypes   map[string]ColumnType
	Converters      map[string]Converter
	EmptyAsNil      bool
	StrictFields    bool
	Loop            bool

	// EventsPerSecond uses the same valid range as Esper's CSVInputAdapter:
	// 1..1000. Zero disables fixed-rate pacing.
	EventsPerSecond int
	// TimestampColumn enables timestamp-delta pacing when EventsPerSecond is
	// zero. Numeric values use TimestampUnit; strings accept integer values or
	// TimestampLayout/RFC3339Nano.
	TimestampColumn string
	TimestampUnit   time.Duration
	TimestampLayout string
	TimeLayout      string

	Comma      rune
	Comment    rune
	LazyQuotes bool
}

type column struct {
	name      string
	index     int
	typ       ColumnType
	converter Converter
}

// Source is a lifecycle-managed CSV source. Source is safe for one consumer;
// concurrent calls to Next are serialized, while Run rejects a second active
// replay loop.
type Source struct {
	manager *connectors.StateManager
	spec    SourceSpec

	mu          sync.Mutex
	reader      *encodingcsv.Reader
	closer      io.Closer
	seeker      io.Seeker
	readSeeker  io.ReadSeeker
	ownedCloser bool
	initialized bool
	headers     []string
	columns     []column
	firstRow    []string
	rowCount    int

	runMu sync.Mutex
}

// NewSource validates and copies a source specification. The underlying
// resource is opened lazily by Start, which keeps construction side-effect
// free and makes lifecycle errors observable from Start.
func NewSource(spec SourceSpec) (*Source, error) {
	if err := validateSourceSpec(spec); err != nil {
		return nil, err
	}
	if spec.TimestampUnit == 0 {
		spec.TimestampUnit = time.Millisecond
	}
	if spec.Comment == 0 {
		// CSVReader ignores lines beginning with '#'; retain that behavior by
		// default while still allowing another comment rune.
		spec.Comment = '#'
	}
	if !spec.AutoDetectTitle && len(spec.PropertyOrder) > 0 && !spec.HasTitleLine {
		// Esper's adapter auto-detects a title row when the first row contains
		// the configured property names. Keep that compatibility behavior as
		// the default while still allowing a caller to force positional data by
		// using a different first row.
		spec.AutoDetectTitle = true
	}
	spec.PropertyOrder = append([]string(nil), spec.PropertyOrder...)
	spec.PropertyTypes = copyColumnTypes(spec.PropertyTypes)
	spec.Converters = copyConverters(spec.Converters)
	return &Source{manager: connectors.NewStateManager(), spec: spec}, nil
}

func validateSourceSpec(spec SourceSpec) error {
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
		return fmt.Errorf("csv: exactly one of Path, Reader, or Open is required")
	}
	if spec.EventsPerSecond < 0 || spec.EventsPerSecond > 1000 {
		return fmt.Errorf("csv: EventsPerSecond must be between 0 and 1000")
	}
	if !spec.HasTitleLine && len(spec.PropertyOrder) == 0 {
		return fmt.Errorf("csv: PropertyOrder is required when HasTitleLine is false")
	}
	if err := validateDelimiter(spec.Comma, spec.Comment); err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(spec.PropertyOrder))
	for _, descriptor := range spec.PropertyOrder {
		name, _ := parseColumnDescriptor(descriptor)
		if name == "" {
			return fmt.Errorf("csv: PropertyOrder contains an empty column")
		}
		if _, exists := seen[name]; exists {
			return fmt.Errorf("csv: PropertyOrder contains duplicate column %q", name)
		}
		seen[name] = struct{}{}
	}
	return nil
}

func validateDelimiter(comma, comment rune) error {
	if comma != 0 && !validDelimiter(comma) {
		return fmt.Errorf("csv: invalid Comma %q", comma)
	}
	if comment != 0 && !validDelimiter(comment) {
		return fmt.Errorf("csv: invalid Comment %q", comment)
	}
	if comma != 0 && comment != 0 && comma == comment {
		return fmt.Errorf("csv: Comma and Comment must differ")
	}
	return nil
}

func validDelimiter(value rune) bool {
	return value != 0 && value != '"' && value != '\r' && value != '\n' && value != utf8.RuneError && utf8.ValidRune(value)
}

func copyColumnTypes(values map[string]ColumnType) map[string]ColumnType {
	if values == nil {
		return nil
	}
	result := make(map[string]ColumnType, len(values))
	for name, typ := range values {
		result[name] = typ
	}
	return result
}

func copyConverters(values map[string]Converter) map[string]Converter {
	if values == nil {
		return nil
	}
	result := make(map[string]Converter, len(values))
	for name, converter := range values {
		result[name] = converter
	}
	return result
}

// State returns the current Java-compatible adapter state.
func (s *Source) State() connectors.State {
	if s == nil {
		return connectors.Destroyed
	}
	return s.manager.State()
}

func (s *Source) Start() error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	if err := s.manager.Start(); err != nil {
		return err
	}
	s.mu.Lock()
	err := s.initializeLocked()
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

func (s *Source) Stop() error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	return s.manager.Stop()
}

func (s *Source) Pause() error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	return s.manager.Pause()
}

func (s *Source) Resume() error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	return s.manager.Resume()
}

func (s *Source) Destroy() error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	if err := s.manager.Destroy(); err != nil {
		return err
	}
	s.mu.Lock()
	err := s.closeLocked()
	s.mu.Unlock()
	return err
}

// Reset moves a resettable source back to its beginning. A path or Open
// function can always be reopened; a caller-owned Reader must implement
// io.Seeker. Reset is allowed in OPENED, STARTED, and PAUSED states.
func (s *Source) Reset() error {
	if s == nil {
		return connectors.ErrDestroyed
	}
	if s.State() == connectors.Destroyed {
		return connectors.ErrDestroyed
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.resetLocked()
}

// RowCount reports the number of data records returned by Next, including
// records returned after a loop reset.
func (s *Source) RowCount() int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rowCount
}

// Next returns one converted record. EOF means that a non-looping source has
// finished; reaching EOF automatically transitions a started source to
// OPENED, matching CSVInputAdapter's stop-on-EOF behavior.
func (s *Source) Next(ctx context.Context) (Record, error) {
	if s == nil {
		return nil, connectors.ErrDestroyed
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := s.manager.RequireStarted(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.initializeLocked(); err != nil {
		return nil, err
	}

	resetOnce := false
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		row, err := s.nextRowLocked()
		if err == io.EOF && s.spec.Loop && !resetOnce {
			resetOnce = true
			if resetErr := s.resetLocked(); resetErr != nil {
				return nil, resetErr
			}
			continue
		}
		if err == io.EOF {
			if s.manager.State() == connectors.Started {
				_ = s.manager.Stop()
			}
			return nil, io.EOF
		}
		if err != nil {
			return nil, err
		}
		record, err := s.convertRowLocked(row)
		if err != nil {
			return nil, err
		}
		s.rowCount++
		return record, nil
	}
}

// Run reads until EOF, cancellation, or an adapter error. It may be started
// before Start; in that case it waits for lifecycle control. A Stop causes a
// clean return, while Destroy and context cancellation are returned.
func (s *Source) Run(ctx context.Context, emit func(context.Context, Record) error) error {
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
		record, err := s.Next(ctx)
		if errors.Is(err, connectors.ErrPaused) || errors.Is(err, connectors.ErrNotStarted) {
			continue
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		due, err := pacer.nextDue(s.spec, record, time.Now())
		if err != nil {
			return err
		}
		if err := s.waitUntilDue(ctx, due); err != nil {
			if errors.Is(err, connectors.ErrStopped) {
				return nil
			}
			return err
		}
		if err := emit(ctx, record); err != nil {
			return err
		}
	}
}

// RunToEngine is the direct Esper bridge. The target event type must accept a
// map-backed underlying value, as with Environment.RegisterMap or a compatible
// JSON/Avro schema.
func (s *Source) RunToEngine(ctx context.Context, engine *esper.Engine, eventType string) error {
	if engine == nil {
		return fmt.Errorf("csv: RunToEngine requires an engine")
	}
	if strings.TrimSpace(eventType) == "" {
		return fmt.Errorf("csv: RunToEngine requires an event type")
	}
	return s.Run(ctx, func(ctx context.Context, record Record) error {
		return engine.Send(ctx, eventType, map[string]any(record))
	})
}

func (s *Source) waitUntilDue(ctx context.Context, due time.Time) error {
	for {
		if err := s.manager.WaitUntilStarted(ctx); err != nil {
			return err
		}
		delay := time.Until(due)
		if delay <= 0 {
			return nil
		}
		timer := time.NewTimer(delay)
		changed := s.manager.Changes()
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

func (s *Source) initializeLocked() error {
	if s.initialized {
		return nil
	}
	if s.reader == nil {
		if err := s.openLocked(); err != nil {
			return err
		}
	}
	first, err := s.reader.Read()
	if err != nil && err != io.EOF {
		return fmt.Errorf("csv: read first record: %w", err)
	}
	if err == io.EOF {
		if s.spec.HasTitleLine && len(s.spec.PropertyOrder) == 0 {
			return fmt.Errorf("csv: title row is missing")
		}
		s.headers = propertyNames(s.spec.PropertyOrder)
		if err := s.buildColumnsLocked(); err != nil {
			return err
		}
		s.initialized = true
		return nil
	}

	useTitle := s.spec.HasTitleLine
	if !useTitle && s.spec.AutoDetectTitle && len(s.spec.PropertyOrder) > 0 {
		useTitle = sameColumnSet(first, propertyNames(s.spec.PropertyOrder))
	}
	if useTitle {
		if len(first) == 0 {
			return fmt.Errorf("csv: title row is empty")
		}
		s.headers = append([]string(nil), first...)
	} else {
		s.headers = propertyNames(s.spec.PropertyOrder)
		s.firstRow = append([]string(nil), first...)
	}
	if err := s.buildColumnsLocked(); err != nil {
		return err
	}
	s.initialized = true
	return nil
}

func (s *Source) buildColumnsLocked() error {
	descriptors := s.spec.PropertyOrder
	if len(descriptors) == 0 {
		descriptors = s.headers
	}
	if len(descriptors) == 0 {
		return fmt.Errorf("csv: no columns were discovered")
	}
	if s.spec.StrictFields && len(s.headers) > 0 && len(s.headers) != len(specNames(descriptors)) {
		return fmt.Errorf("csv: expected %d columns, got %d", len(descriptors), len(s.headers))
	}

	headerIndexes := make(map[string]int, len(s.headers))
	for index, header := range s.headers {
		name := strings.TrimSpace(header)
		if name == "" {
			return fmt.Errorf("csv: title row contains an empty column name")
		}
		if _, exists := headerIndexes[name]; exists {
			return fmt.Errorf("csv: title row contains duplicate column %q", name)
		}
		headerIndexes[name] = index
	}
	columns := make([]column, 0, len(descriptors))
	seen := make(map[string]struct{}, len(descriptors))
	for index, descriptor := range descriptors {
		name, impliedType := parseColumnDescriptor(descriptor)
		if name == "" {
			return fmt.Errorf("csv: column %d has an empty name", index)
		}
		if _, exists := seen[name]; exists {
			return fmt.Errorf("csv: duplicate column %q", name)
		}
		seen[name] = struct{}{}
		rowIndex := index
		if headerIndex, exists := headerIndexes[name]; exists && (s.spec.HasTitleLine || len(s.firstRow) == 0) {
			rowIndex = headerIndex
		}
		typ := impliedType
		if configured, exists := s.spec.PropertyTypes[name]; exists {
			typ = configured
		}
		columns = append(columns, column{
			name:      name,
			index:     rowIndex,
			typ:       typ,
			converter: s.spec.Converters[name],
		})
	}
	s.columns = columns
	return nil
}

func (s *Source) nextRowLocked() ([]string, error) {
	if s.firstRow != nil {
		row := s.firstRow
		s.firstRow = nil
		return row, nil
	}
	row, err := s.reader.Read()
	if err == io.EOF {
		return nil, io.EOF
	}
	if err != nil {
		return nil, fmt.Errorf("csv: read record: %w", err)
	}
	return row, nil
}

func (s *Source) convertRowLocked(row []string) (Record, error) {
	if s.spec.StrictFields && len(row) != len(s.headers) && len(s.headers) > 0 {
		return nil, fmt.Errorf("csv: expected %d fields, got %d", len(s.headers), len(row))
	}
	record := make(Record, len(s.columns))
	for _, column := range s.columns {
		if column.index >= len(row) {
			continue
		}
		raw := row[column.index]
		if raw == "" && s.spec.EmptyAsNil {
			record[column.name] = nil
			continue
		}
		value, err := coerce(raw, column.typ, column.converter, s.spec.TimeLayout)
		if err != nil {
			return nil, fmt.Errorf("csv: column %q value %q: %w", column.name, raw, err)
		}
		record[column.name] = value
	}
	return record, nil
}

func (s *Source) openLocked() error {
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
			return fmt.Errorf("csv: open source: %w", err)
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
	s.reader = newCSVReader(reader, s.spec)
	s.closer = closer
	s.seeker = seeker
	s.readSeeker = readSeeker
	s.ownedCloser = owned
	return nil
}

func newCSVReader(reader io.Reader, spec SourceSpec) *encodingcsv.Reader {
	result := encodingcsv.NewReader(reader)
	result.FieldsPerRecord = -1
	result.TrimLeadingSpace = true
	result.ReuseRecord = false
	result.LazyQuotes = spec.LazyQuotes
	if spec.Comma != 0 {
		result.Comma = spec.Comma
	}
	if spec.Comment != 0 {
		result.Comment = spec.Comment
	}
	return result
}

func (s *Source) resetLocked() error {
	if s.reader == nil {
		return nil
	}
	if s.readSeeker != nil {
		if _, err := s.readSeeker.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("csv: reset source: %w", err)
		}
		s.reader = newCSVReader(s.readSeeker, s.spec)
	} else if strings.TrimSpace(s.spec.Path) != "" || s.spec.Open != nil {
		if err := s.closeLocked(); err != nil {
			return fmt.Errorf("csv: close source for reset: %w", err)
		}
		if err := s.openLocked(); err != nil {
			return err
		}
	} else {
		return fmt.Errorf("csv: source is not resettable")
	}
	s.initialized = false
	s.headers = nil
	s.columns = nil
	s.firstRow = nil
	return s.initializeLocked()
}

func (s *Source) closeLocked() error {
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

func propertyNames(descriptors []string) []string {
	if len(descriptors) == 0 {
		return nil
	}
	result := make([]string, len(descriptors))
	for index, descriptor := range descriptors {
		result[index], _ = parseColumnDescriptor(descriptor)
	}
	return result
}

func specNames(descriptors []string) []string { return propertyNames(descriptors) }

func parseColumnDescriptor(descriptor string) (string, ColumnType) {
	parts := strings.Fields(descriptor)
	if len(parts) == 0 {
		return "", ColumnTypeString
	}
	if len(parts) == 2 {
		if typ, ok := namedColumnType(parts[0]); ok {
			return parts[1], typ
		}
	}
	return strings.TrimSpace(descriptor), ColumnTypeString
}

func namedColumnType(value string) (ColumnType, bool) {
	switch strings.ToLower(value) {
	case "string", "str":
		return ColumnTypeString, true
	case "int", "integer", "int32":
		return ColumnTypeInt, true
	case "long", "int64":
		return ColumnTypeInt64, true
	case "double", "float", "float64", "number":
		return ColumnTypeFloat64, true
	case "bool", "boolean":
		return ColumnTypeBool, true
	case "date", "datetime", "time", "timestamp":
		return ColumnTypeTime, true
	case "json", "object":
		return ColumnTypeJSON, true
	default:
		return ColumnTypeString, false
	}
}

func sameColumnSet(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	seen := make(map[string]struct{}, len(left))
	for _, value := range left {
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
	}
	for _, value := range right {
		if _, exists := seen[value]; !exists {
			return false
		}
	}
	return true
}

func coerce(raw string, typ ColumnType, converter Converter, timeLayout string) (any, error) {
	if converter != nil {
		return converter(raw)
	}
	switch typ {
	case ColumnTypeString:
		return raw, nil
	case ColumnTypeInt:
		value, err := strconv.Atoi(strings.TrimSpace(raw))
		return value, err
	case ColumnTypeInt64:
		value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
		return value, err
	case ColumnTypeFloat64:
		value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
		return value, err
	case ColumnTypeBool:
		value, err := strconv.ParseBool(strings.TrimSpace(raw))
		return value, err
	case ColumnTypeTime:
		if timeLayout == "" {
			timeLayout = time.RFC3339Nano
		}
		value, err := time.Parse(timeLayout, strings.TrimSpace(raw))
		return value, err
	case ColumnTypeJSON:
		var value any
		decoder := json.NewDecoder(strings.NewReader(raw))
		decoder.UseNumber()
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported column type %d", typ)
	}
}

type replayPacer struct {
	initialized   bool
	lastTimestamp int64
	lastDue       time.Time
}

func (p *replayPacer) nextDue(spec SourceSpec, record Record, now time.Time) (time.Time, error) {
	if spec.EventsPerSecond > 0 {
		interval := time.Second / time.Duration(spec.EventsPerSecond)
		if !p.initialized {
			p.initialized = true
			p.lastDue = now.Add(interval)
		} else {
			p.lastDue = p.lastDue.Add(interval)
		}
		return p.lastDue, nil
	}
	if strings.TrimSpace(spec.TimestampColumn) == "" {
		return now, nil
	}
	raw, exists := record[spec.TimestampColumn]
	if !exists || raw == nil {
		return time.Time{}, fmt.Errorf("csv: timestamp column %q is missing or null", spec.TimestampColumn)
	}
	timestamp, err := timestampNanos(raw, spec)
	if err != nil {
		return time.Time{}, fmt.Errorf("csv: timestamp column %q: %w", spec.TimestampColumn, err)
	}
	if timestamp < 0 {
		return time.Time{}, fmt.Errorf("csv: timestamp cannot be negative")
	}
	if !p.initialized {
		p.initialized = true
		p.lastTimestamp = timestamp
		p.lastDue = now.Add(time.Duration(timestamp))
		return p.lastDue, nil
	}
	if timestamp < p.lastTimestamp {
		return time.Time{}, fmt.Errorf("csv: timestamp %d is smaller than previous timestamp %d", timestamp, p.lastTimestamp)
	}
	p.lastDue = p.lastDue.Add(time.Duration(timestamp - p.lastTimestamp))
	p.lastTimestamp = timestamp
	return p.lastDue, nil
}

func timestampNanos(value any, spec SourceSpec) (int64, error) {
	switch typed := value.(type) {
	case time.Time:
		return typed.UnixNano(), nil
	case int:
		return multiplyTimestamp(int64(typed), spec.TimestampUnit)
	case int8:
		return multiplyTimestamp(int64(typed), spec.TimestampUnit)
	case int16:
		return multiplyTimestamp(int64(typed), spec.TimestampUnit)
	case int32:
		return multiplyTimestamp(int64(typed), spec.TimestampUnit)
	case int64:
		return multiplyTimestamp(typed, spec.TimestampUnit)
	case uint:
		return multiplyTimestampUnsigned(uint64(typed), spec.TimestampUnit)
	case uint8:
		return multiplyTimestampUnsigned(uint64(typed), spec.TimestampUnit)
	case uint16:
		return multiplyTimestampUnsigned(uint64(typed), spec.TimestampUnit)
	case uint32:
		return multiplyTimestampUnsigned(uint64(typed), spec.TimestampUnit)
	case uint64:
		return multiplyTimestampUnsigned(typed, spec.TimestampUnit)
	case float32:
		return multiplyTimestampFloat(float64(typed), spec.TimestampUnit)
	case float64:
		return multiplyTimestampFloat(typed, spec.TimestampUnit)
	case string:
		text := strings.TrimSpace(typed)
		if integer, err := strconv.ParseInt(text, 10, 64); err == nil {
			return multiplyTimestamp(integer, spec.TimestampUnit)
		}
		layout := spec.TimestampLayout
		if layout == "" {
			layout = time.RFC3339Nano
		}
		parsed, err := time.Parse(layout, text)
		if err != nil {
			return 0, err
		}
		return parsed.UnixNano(), nil
	default:
		return 0, fmt.Errorf("unsupported timestamp type %T", value)
	}
}

func multiplyTimestamp(value int64, unit time.Duration) (int64, error) {
	if unit <= 0 {
		return 0, fmt.Errorf("TimestampUnit must be positive")
	}
	if value > int64(time.Duration(1<<63-1))/int64(unit) || value < int64(time.Duration(-1<<63))/int64(unit) {
		return 0, fmt.Errorf("timestamp overflows duration")
	}
	return value * int64(unit), nil
}

func multiplyTimestampUnsigned(value uint64, unit time.Duration) (int64, error) {
	if unit <= 0 || value > uint64(time.Duration(1<<63-1))/uint64(unit) {
		return 0, fmt.Errorf("timestamp overflows duration")
	}
	return int64(value) * int64(unit), nil
}

func multiplyTimestampFloat(value float64, unit time.Duration) (int64, error) {
	if value < 0 || value > float64(time.Duration(1<<63-1))/float64(unit) {
		return 0, fmt.Errorf("timestamp overflows duration")
	}
	return int64(value * float64(unit)), nil
}

// EventToRecord converts common Esper result/event values into a sink record.
// It is exported so dataflow integrations can share the exact column mapping.
func EventToRecord(value any) (Record, error) {
	switch typed := value.(type) {
	case Record:
		return copyRecord(typed), nil
	case map[string]any:
		return copyRecord(Record(typed)), nil
	case esper.Event:
		record := make(Record, len(typed.Schema().Fields()))
		for _, field := range typed.Schema().Fields() {
			property := typed.Get(field.Name)
			if property.IsPresent() || property.IsNull() {
				record[field.Name] = property.Any()
			}
		}
		return record, nil
	case esper.Row:
		return Record(typed.AsMap()), nil
	case esper.TableRow:
		record := make(Record)
		for name, property := range typed.Values() {
			if property.IsPresent() || property.IsNull() {
				record[name] = property.Any()
			}
		}
		return record, nil
	default:
		return structToRecord(value)
	}
}

func copyRecord(value Record) Record {
	result := make(Record, len(value))
	for name, item := range value {
		result[name] = item
	}
	return result
}
