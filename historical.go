package esper

import (
	"container/list"
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
)

// HistoricalRequest is the input available to an external lookup. A
// provider can bind fields from Trigger or variables to a prepared query.
type HistoricalRequest struct {
	Trigger    Event
	Now        time.Time
	Variables  map[string]Value
	Parameters map[string]Value
}

// HistoricalProvider is the external lookup contract used by
// FromHistorical. It is deliberately driver-neutral; SQL drivers are
// supplied by the host through database/sql.
type HistoricalProvider interface {
	Poll(context.Context, HistoricalRequest) ([]Event, error)
}

// SQLHistoricalQueryer is implemented by *sql.DB and *sql.Tx. It lets a
// historical provider participate in a caller-owned transaction without
// making the provider responsible for commit or rollback.
type SQLHistoricalQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

type sqlHistoricalPreparer interface {
	PrepareContext(context.Context, string) (*sql.Stmt, error)
}

// SQLColumnCase controls an optional normalization applied to result column
// names before looking them up in the historical event schema. It is useful
// with databases or drivers that expose unquoted identifiers in a fixed case.
type SQLColumnCase uint8

const (
	SQLColumnCasePreserve SQLColumnCase = iota
	SQLColumnCaseLower
	SQLColumnCaseUpper
)

// SQLHistoricalCacheConfig selects the result cache used by a SQL historical
// provider. LRUSize and MaxAge are mutually exclusive, matching Esper's
// database reference configuration. A zero value disables caching.
type SQLHistoricalCacheConfig struct {
	LRUSize       int
	MaxAge        time.Duration
	PurgeInterval time.Duration
}

// SQLHistoricalColumnMetadata describes the metadata exposed by a database
// driver for one result column. The schema remains the source of the event
// field type; metadata is supplied to custom conversion hooks for drivers
// that need SQL-type-specific handling.
type SQLHistoricalColumnMetadata struct {
	OriginalName     string
	Name             string
	DatabaseTypeName string
	ScanType         reflect.Type
}

// SQLHistoricalMetadataMode controls how a driver result's ColumnTypes
// metadata is used. Default is best-effort, Required returns metadata errors,
// and Skip avoids metadata interrogation entirely.
type SQLHistoricalMetadataMode uint8

const (
	SQLHistoricalMetadataDefault SQLHistoricalMetadataMode = iota
	SQLHistoricalMetadataRequired
	SQLHistoricalMetadataSkip
)

// SQLHistoricalValueConverter can override the default database/sql to Go
// conversion for a result column. The returned value is stored as-is in the
// map-backed historical event.
type SQLHistoricalValueConverter func(SQLHistoricalColumnMetadata, any, reflect.Type) (any, error)

// SQLHistoricalStatementRewriter adapts a statement to a driver's placeholder
// convention. It is called once when the provider is constructed.
type SQLHistoricalStatementRewriter func(statement string, argumentCount int) (string, error)

// NewSQLPositionalPlaceholderRewriter returns a rewriter that changes
// question-mark placeholders outside quoted SQL literals to prefix+n, such as
// $1/$2 for PostgreSQL. A positive startAt is required.
func NewSQLPositionalPlaceholderRewriter(prefix string, startAt int) SQLHistoricalStatementRewriter {
	return func(statement string, argumentCount int) (string, error) {
		return rewriteSQLQuestionPlaceholders(statement, argumentCount, prefix, startAt)
	}
}

func (c SQLHistoricalCacheConfig) validate() error {
	if c.LRUSize < 0 {
		return NewError(ErrorInvalidRule, "historical SQL LRU cache size cannot be negative")
	}
	if c.MaxAge < 0 {
		return NewError(ErrorInvalidRule, "historical SQL cache maximum age cannot be negative")
	}
	if c.PurgeInterval < 0 {
		return NewError(ErrorInvalidRule, "historical SQL cache purge interval cannot be negative")
	}
	if c.LRUSize > 0 && c.MaxAge > 0 {
		return NewError(ErrorInvalidRule, "historical SQL cache cannot configure both LRU and expiry")
	}
	if c.MaxAge > 0 && c.PurgeInterval == 0 {
		return NewError(ErrorInvalidRule, "historical SQL expiry cache requires a positive purge interval")
	}
	return nil
}

// SQLHistoricalProviderOptions configures a SQLHistoricalProvider without
// changing the original constructor that accepts argument functions.
type SQLHistoricalProviderOptions struct {
	Arguments         []func(HistoricalRequest) any
	Cache             SQLHistoricalCacheConfig
	ColumnCase        SQLColumnCase
	ValueConverter    SQLHistoricalValueConverter
	StatementRewriter SQLHistoricalStatementRewriter
	PrepareStatement  bool
	MetadataMode      SQLHistoricalMetadataMode
	TypeBindings      map[string]reflect.Type
}

type historicalDefinition struct {
	name     string
	trigger  string
	schema   Schema
	provider HistoricalProvider
}

// SQLHistoricalProvider adapts a prepared database/sql query to a historical
// stream. The caller owns the DB pool and chooses the database driver. Each
// argument function is evaluated for the current trigger event.
type SQLHistoricalProvider struct {
	DB        *sql.DB
	Schema    Schema
	Statement string
	Arguments []func(HistoricalRequest) any

	queryer      SQLHistoricalQueryer
	columnCase   SQLColumnCase
	converter    SQLHistoricalValueConverter
	typeBindings map[string]reflect.Type
	metadataMode SQLHistoricalMetadataMode
	prepare      bool
	statement    *sql.Stmt
	statementMu  sync.Mutex
	closed       bool
	cache        *historicalSQLCache
	cacheMu      sync.Mutex
}

func NewSQLHistoricalProvider(db *sql.DB, schema Schema, statement string, arguments ...func(HistoricalRequest) any) (*SQLHistoricalProvider, error) {
	return NewSQLHistoricalProviderWithQueryer(db, schema, statement, SQLHistoricalProviderOptions{Arguments: arguments})
}

// NewSQLHistoricalProviderWithOptions constructs a database/sql historical
// provider with optional result caching and result-column normalization.
func NewSQLHistoricalProviderWithOptions(db *sql.DB, schema Schema, statement string, options SQLHistoricalProviderOptions) (*SQLHistoricalProvider, error) {
	return NewSQLHistoricalProviderWithQueryer(db, schema, statement, options)
}

// NewSQLHistoricalProviderWithQueryer constructs a provider over either a
// database pool or a caller-owned transaction. The provider never commits,
// rolls back, or closes the queryer.
func NewSQLHistoricalProviderWithQueryer(queryer SQLHistoricalQueryer, schema Schema, statement string, options SQLHistoricalProviderOptions) (*SQLHistoricalProvider, error) {
	if queryer == nil {
		return nil, NewError(ErrorDependency, "historical SQL provider requires a database")
	}
	if !schema.valid() {
		return nil, NewError(ErrorInvalidRule, "historical SQL provider requires a schema")
	}
	if schema.Kind() == SchemaStruct || schema.GoType() != nil {
		return nil, NewError(ErrorTypeMismatch, "historical SQL provider requires a map-backed schema")
	}
	if strings.TrimSpace(statement) == "" {
		return nil, NewError(ErrorInvalidRule, "historical SQL statement is required")
	}
	if options.ColumnCase != SQLColumnCasePreserve && options.ColumnCase != SQLColumnCaseLower && options.ColumnCase != SQLColumnCaseUpper {
		return nil, NewError(ErrorInvalidRule, "historical SQL column case is invalid")
	}
	if options.MetadataMode != SQLHistoricalMetadataDefault && options.MetadataMode != SQLHistoricalMetadataRequired && options.MetadataMode != SQLHistoricalMetadataSkip {
		return nil, NewError(ErrorInvalidRule, "historical SQL metadata mode is invalid")
	}
	if err := options.Cache.validate(); err != nil {
		return nil, err
	}
	preparedStatement := statement
	if options.StatementRewriter != nil {
		var rewriteErr error
		preparedStatement, rewriteErr = options.StatementRewriter(statement, len(options.Arguments))
		if rewriteErr != nil {
			return nil, WrapError(ErrorInvalidRule, "historical SQL statement", rewriteErr)
		}
		if strings.TrimSpace(preparedStatement) == "" {
			return nil, NewError(ErrorInvalidRule, "historical SQL statement rewriter returned an empty statement")
		}
	}
	if options.PrepareStatement {
		if _, ok := queryer.(sqlHistoricalPreparer); !ok {
			return nil, NewError(ErrorDependency, "historical SQL provider queryer does not support prepared statements")
		}
	}
	var db *sql.DB
	if typedDB, ok := queryer.(*sql.DB); ok {
		db = typedDB
	}
	return &SQLHistoricalProvider{
		DB:           db,
		Schema:       schema,
		Statement:    preparedStatement,
		Arguments:    append([]func(HistoricalRequest) any(nil), options.Arguments...),
		queryer:      queryer,
		columnCase:   options.ColumnCase,
		converter:    options.ValueConverter,
		typeBindings: cloneHistoricalTypeBindings(options.TypeBindings),
		metadataMode: options.MetadataMode,
		prepare:      options.PrepareStatement,
		cache:        newHistoricalSQLCache(options.Cache),
	}, nil
}

// ConfigureCache replaces the provider cache. Configure a provider before it
// is shared with a running engine; the cache itself is safe for concurrent
// Poll calls after configuration.
func (p *SQLHistoricalProvider) ConfigureCache(config SQLHistoricalCacheConfig) error {
	if p == nil {
		return NewError(ErrorDependency, "historical SQL provider is nil")
	}
	if err := config.validate(); err != nil {
		return err
	}
	p.cacheMu.Lock()
	p.cache = newHistoricalSQLCache(config)
	p.cacheMu.Unlock()
	return nil
}

// Close releases a lazily prepared statement. The caller still owns DB and
// must close it separately. A provider without PrepareStatement is a no-op.
func (p *SQLHistoricalProvider) Close() error {
	if p == nil {
		return nil
	}
	p.statementMu.Lock()
	p.closed = true
	statement := p.statement
	p.statement = nil
	p.statementMu.Unlock()
	if statement == nil {
		return nil
	}
	return statement.Close()
}

func (p *SQLHistoricalProvider) Poll(ctx context.Context, request HistoricalRequest) ([]Event, error) {
	if p == nil || (p.DB == nil && p.queryer == nil) {
		return nil, NewError(ErrorDependency, "historical SQL provider has no database")
	}
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	p.statementMu.Lock()
	if p.closed {
		p.statementMu.Unlock()
		return nil, NewError(ErrorDependency, "historical SQL provider is closed")
	}
	p.statementMu.Unlock()
	args := make([]any, 0, len(p.Arguments))
	for _, argument := range p.Arguments {
		if argument == nil {
			args = append(args, nil)
			continue
		}
		args = append(args, argument(request))
	}
	cacheKey := encodeKey(args)
	cacheNow := request.Now
	if cacheNow.IsZero() {
		cacheNow = time.Now()
	}
	p.cacheMu.Lock()
	if cached, ok := p.cache.get(cacheKey, cacheNow); ok {
		p.cacheMu.Unlock()
		return cached, nil
	}
	p.cacheMu.Unlock()
	rows, err := p.query(ctx, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	var columnTypes []*sql.ColumnType
	if p.metadataMode != SQLHistoricalMetadataSkip {
		columnTypes, err = rows.ColumnTypes()
		if err != nil && p.metadataMode == SQLHistoricalMetadataRequired {
			return nil, err
		}
		if err != nil {
			columnTypes = nil
		}
	}
	metadata := make([]SQLHistoricalColumnMetadata, len(columns))
	for index, column := range columns {
		metadata[index] = SQLHistoricalColumnMetadata{OriginalName: column, Name: normalizeSQLColumn(column, p.columnCase)}
		if index < len(columnTypes) {
			metadata[index].DatabaseTypeName = columnTypes[index].DatabaseTypeName()
			metadata[index].ScanType = columnTypes[index].ScanType()
		}
	}
	result := make([]Event, 0)
	seenFields := make(map[string]string, len(columns))
	for rows.Next() {
		values := make([]any, len(columns))
		destinations := make([]any, len(columns))
		for index := range values {
			destinations[index] = &values[index]
		}
		if err := rows.Scan(destinations...); err != nil {
			return nil, err
		}
		underlying := make(map[string]any, len(columns))
		for index, column := range columns {
			lookupColumn := metadata[index].Name
			field, ok := p.Schema.Field(lookupColumn)
			if !ok {
				return nil, NewError(ErrorUnknownName, fmt.Sprintf("historical SQL column %q is absent from schema %q", lookupColumn, p.Schema.Name()))
			}
			if previous, duplicate := seenFields[field.Name]; duplicate {
				return nil, NewError(ErrorInvalidRule, fmt.Sprintf("historical SQL columns %q and %q both map to schema field %q", previous, column, field.Name))
			}
			seenFields[field.Name] = column
			targetType := field.Type
			if binding, bound := p.typeBindings[strings.ToUpper(metadata[index].DatabaseTypeName)]; bound {
				targetType = binding
			}
			var coerced any
			var coerceErr error
			if p.converter != nil {
				coerced, coerceErr = p.converter(metadata[index], values[index], targetType)
			} else {
				coerced, coerceErr = coerceHistoricalSQLValue(values[index], targetType)
			}
			if coerceErr != nil {
				return nil, fmt.Errorf("historical SQL column %q: %w", column, coerceErr)
			}
			underlying[field.Name] = coerced
		}
		event, eventErr := newEvent(p.Schema, underlying, request.Now)
		if eventErr != nil {
			return nil, eventErr
		}
		result = append(result, event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	p.cacheMu.Lock()
	p.cache.put(cacheKey, result, cacheNow)
	p.cacheMu.Unlock()
	return result, nil
}

func cloneHistoricalTypeBindings(bindings map[string]reflect.Type) map[string]reflect.Type {
	if len(bindings) == 0 {
		return nil
	}
	cloned := make(map[string]reflect.Type, len(bindings))
	for name, target := range bindings {
		if strings.TrimSpace(name) == "" || target == nil {
			continue
		}
		cloned[strings.ToUpper(strings.TrimSpace(name))] = target
	}
	return cloned
}

func (p *SQLHistoricalProvider) query(ctx context.Context, args ...any) (*sql.Rows, error) {
	p.statementMu.Lock()
	if p.closed {
		p.statementMu.Unlock()
		return nil, NewError(ErrorDependency, "historical SQL provider is closed")
	}
	if !p.prepare {
		p.statementMu.Unlock()
		if p.queryer != nil {
			return p.queryer.QueryContext(ctx, p.Statement, args...)
		}
		return p.DB.QueryContext(ctx, p.Statement, args...)
	}
	if p.statement == nil {
		preparer, ok := p.queryer.(sqlHistoricalPreparer)
		if !ok && p.DB != nil {
			preparer, ok = any(p.DB).(sqlHistoricalPreparer)
		}
		if !ok {
			p.statementMu.Unlock()
			return nil, NewError(ErrorDependency, "historical SQL provider queryer does not support prepared statements")
		}
		statement, err := preparer.PrepareContext(ctx, p.Statement)
		if err != nil {
			p.statementMu.Unlock()
			return nil, err
		}
		p.statement = statement
	}
	statement := p.statement
	p.statementMu.Unlock()
	return statement.QueryContext(ctx, args...)
}

func rewriteSQLQuestionPlaceholders(statement string, argumentCount int, prefix string, startAt int) (string, error) {
	if startAt <= 0 {
		return "", NewError(ErrorInvalidRule, "SQL placeholder numbering must start at a positive index")
	}
	if prefix == "" {
		return "", NewError(ErrorInvalidRule, "SQL placeholder prefix is required")
	}
	var builder strings.Builder
	builder.Grow(len(statement) + argumentCount*2)
	inSingleQuote := false
	inDoubleQuote := false
	inBacktick := false
	count := 0
	for index := 0; index < len(statement); index++ {
		character := statement[index]
		if character == '\\' && (inSingleQuote || inDoubleQuote) && index+1 < len(statement) {
			builder.WriteByte(character)
			index++
			builder.WriteByte(statement[index])
			continue
		}
		switch character {
		case '\'':
			if !inDoubleQuote && !inBacktick {
				if inSingleQuote && index+1 < len(statement) && statement[index+1] == '\'' {
					builder.WriteByte(character)
					index++
					builder.WriteByte(statement[index])
					continue
				}
				inSingleQuote = !inSingleQuote
			}
		case '"':
			if !inSingleQuote && !inBacktick {
				if inDoubleQuote && index+1 < len(statement) && statement[index+1] == '"' {
					builder.WriteByte(character)
					index++
					builder.WriteByte(statement[index])
					continue
				}
				inDoubleQuote = !inDoubleQuote
			}
		case '`':
			if !inSingleQuote && !inDoubleQuote {
				inBacktick = !inBacktick
			}
		case '?':
			if !inSingleQuote && !inDoubleQuote && !inBacktick {
				count++
				builder.WriteString(prefix)
				builder.WriteString(strconv.Itoa(startAt + count - 1))
				continue
			}
		}
		builder.WriteByte(character)
	}
	if count != argumentCount {
		return "", fmt.Errorf("SQL statement contains %d placeholders but %d arguments were supplied", count, argumentCount)
	}
	return builder.String(), nil
}

func normalizeSQLColumn(column string, columnCase SQLColumnCase) string {
	switch columnCase {
	case SQLColumnCaseLower:
		return strings.ToLower(column)
	case SQLColumnCaseUpper:
		return strings.ToUpper(column)
	default:
		return column
	}
}

type historicalSQLCacheItem struct {
	key       string
	events    []Event
	storedAt  time.Time
	listEntry *list.Element
}

type historicalSQLCache struct {
	config      SQLHistoricalCacheConfig
	items       map[string]*historicalSQLCacheItem
	order       *list.List
	lastPurgeAt time.Time
}

func newHistoricalSQLCache(config SQLHistoricalCacheConfig) *historicalSQLCache {
	return &historicalSQLCache{
		config: config,
		items:  make(map[string]*historicalSQLCacheItem),
		order:  list.New(),
	}
}

func (c *historicalSQLCache) enabled() bool {
	return c != nil && (c.config.LRUSize > 0 || c.config.MaxAge > 0)
}

func (c *historicalSQLCache) get(key string, now time.Time) ([]Event, bool) {
	if !c.enabled() {
		return nil, false
	}
	c.purge(now)
	item, ok := c.items[key]
	if !ok {
		return nil, false
	}
	if c.config.MaxAge > 0 && historicalAge(now, item.storedAt) > c.config.MaxAge {
		c.remove(item)
		return nil, false
	}
	if c.config.LRUSize > 0 {
		c.order.MoveToFront(item.listEntry)
	}
	return cloneHistoricalEvents(item.events), true
}

func (c *historicalSQLCache) put(key string, events []Event, now time.Time) {
	if !c.enabled() {
		return
	}
	c.purge(now)
	if item, exists := c.items[key]; exists {
		item.events = cloneHistoricalEvents(events)
		item.storedAt = now
		if c.config.LRUSize > 0 {
			c.order.MoveToFront(item.listEntry)
		}
		return
	}
	item := &historicalSQLCacheItem{key: key, events: cloneHistoricalEvents(events), storedAt: now}
	if c.config.LRUSize > 0 {
		item.listEntry = c.order.PushFront(item)
	}
	c.items[key] = item
	if c.config.LRUSize > 0 && c.order.Len() > c.config.LRUSize {
		oldest := c.order.Back()
		if oldest != nil {
			c.remove(oldest.Value.(*historicalSQLCacheItem))
		}
	}
}

func (c *historicalSQLCache) purge(now time.Time) {
	if c == nil || c.config.MaxAge <= 0 {
		return
	}
	if c.config.PurgeInterval > 0 && !c.lastPurgeAt.IsZero() && historicalAge(now, c.lastPurgeAt) < c.config.PurgeInterval {
		return
	}
	for _, item := range c.items {
		if historicalAge(now, item.storedAt) > c.config.MaxAge {
			c.remove(item)
		}
	}
	c.lastPurgeAt = now
}

func (c *historicalSQLCache) remove(item *historicalSQLCacheItem) {
	if item == nil {
		return
	}
	delete(c.items, item.key)
	if item.listEntry != nil {
		c.order.Remove(item.listEntry)
		item.listEntry = nil
	}
}

func historicalAge(now, then time.Time) time.Duration {
	if now.Before(then) {
		return 0
	}
	return now.Sub(then)
}

func cloneHistoricalEvents(events []Event) []Event {
	if events == nil {
		return nil
	}
	cloned := make([]Event, len(events))
	for index, event := range events {
		cloned[index] = event
		cloned[index].underlying = cloneHistoricalValue(event.Underlying())
	}
	return cloned
}

func cloneHistoricalValue(value any) any {
	if value == nil {
		return nil
	}
	return cloneHistoricalReflect(reflect.ValueOf(value)).Interface()
}

func cloneHistoricalReflect(value reflect.Value) reflect.Value {
	if !value.IsValid() {
		return value
	}
	switch value.Kind() {
	case reflect.Interface:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		cloned := cloneHistoricalReflect(value.Elem())
		result := reflect.New(value.Type()).Elem()
		result.Set(cloned)
		return result
	case reflect.Map:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		result := reflect.MakeMapWithSize(value.Type(), value.Len())
		iter := value.MapRange()
		for iter.Next() {
			result.SetMapIndex(iter.Key(), cloneHistoricalReflect(iter.Value()))
		}
		return result
	case reflect.Slice:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		result := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		reflect.Copy(result, value)
		if value.Type().Elem().Kind() == reflect.Interface || value.Type().Elem().Kind() == reflect.Map || value.Type().Elem().Kind() == reflect.Slice || value.Type().Elem().Kind() == reflect.Pointer {
			for index := 0; index < value.Len(); index++ {
				result.Index(index).Set(cloneHistoricalReflect(value.Index(index)))
			}
		}
		return result
	case reflect.Array:
		result := reflect.New(value.Type()).Elem()
		for index := 0; index < value.Len(); index++ {
			result.Index(index).Set(cloneHistoricalReflect(value.Index(index)))
		}
		return result
	case reflect.Pointer:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		result := reflect.New(value.Type().Elem())
		result.Elem().Set(cloneHistoricalReflect(value.Elem()))
		return result
	default:
		return value
	}
}

func coerceHistoricalSQLValue(value any, target reflect.Type) (any, error) {
	if value == nil || target == nil {
		return value, nil
	}
	if target.Kind() == reflect.Interface {
		return value, nil
	}
	if reflect.TypeOf(value).AssignableTo(target) {
		return value, nil
	}
	if target.Kind() == reflect.Pointer {
		coerced, err := coerceHistoricalSQLValue(value, target.Elem())
		if err != nil || coerced == nil {
			return coerced, err
		}
		pointer := reflect.New(target.Elem())
		converted, convertErr := assignHistoricalReflectValue(pointer.Elem(), coerced)
		if convertErr != nil {
			return nil, convertErr
		}
		pointer.Elem().Set(converted)
		return pointer.Interface(), nil
	}
	raw, isBytes := value.([]byte)
	textValue := ""
	if isBytes {
		textValue = string(raw)
	} else if text, ok := value.(string); ok {
		textValue = text
	}
	switch target.Kind() {
	case reflect.String:
		if isBytes {
			return textValue, nil
		}
		if text, ok := value.(string); ok {
			return text, nil
		}
		return fmt.Sprint(value), nil
	case reflect.Bool:
		if parsed, ok := value.(bool); ok {
			return parsed, nil
		}
		if isBytes || textValue != "" {
			parsed, err := strconv.ParseBool(textValue)
			if err == nil {
				return parsed, nil
			}
			if textValue == "0" {
				return false, nil
			}
			if textValue == "1" {
				return true, nil
			}
			return nil, err
		}
		if numeric, ok := historicalNumericFloat(value); ok {
			return numeric != 0, nil
		}
		return nil, fmt.Errorf("cannot convert %T to bool", value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		parsed, err := historicalNumericInt64(value)
		if err != nil {
			return nil, err
		}
		converted := reflect.New(target).Elem()
		converted.SetInt(parsed)
		return converted.Interface(), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		parsed, err := historicalNumericUint64(value)
		if err != nil {
			return nil, err
		}
		converted := reflect.New(target).Elem()
		converted.SetUint(parsed)
		return converted.Interface(), nil
	case reflect.Float32, reflect.Float64:
		parsed, err := historicalNumericFloat64(value)
		if err != nil {
			return nil, err
		}
		converted := reflect.New(target).Elem()
		converted.SetFloat(parsed)
		return converted.Interface(), nil
	default:
		return value, nil
	}
}

func assignHistoricalReflectValue(target reflect.Value, value any) (reflect.Value, error) {
	if value == nil {
		return reflect.Zero(target.Type()), nil
	}
	got := reflect.ValueOf(value)
	if got.Type().AssignableTo(target.Type()) {
		return got, nil
	}
	if got.Type().ConvertibleTo(target.Type()) {
		return got.Convert(target.Type()), nil
	}
	return reflect.Value{}, fmt.Errorf("expects %s, got %s", target.Type(), got.Type())
}

func historicalNumericInt64(value any) (int64, error) {
	if raw, ok := value.([]byte); ok {
		return strconv.ParseInt(string(raw), 10, 64)
	}
	if text, ok := value.(string); ok {
		return strconv.ParseInt(text, 10, 64)
	}
	rawValue := reflect.ValueOf(value)
	if rawValue.IsValid() {
		switch rawValue.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return rawValue.Int(), nil
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return int64(rawValue.Uint()), nil
		case reflect.Float32, reflect.Float64:
			return int64(rawValue.Float()), nil
		}
	}
	return 0, fmt.Errorf("cannot convert %T to integer", value)
}

func historicalNumericUint64(value any) (uint64, error) {
	if raw, ok := value.([]byte); ok {
		return strconv.ParseUint(string(raw), 10, 64)
	}
	if text, ok := value.(string); ok {
		return strconv.ParseUint(text, 10, 64)
	}
	rawValue := reflect.ValueOf(value)
	if rawValue.IsValid() {
		switch rawValue.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return uint64(rawValue.Int()), nil
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return rawValue.Uint(), nil
		case reflect.Float32, reflect.Float64:
			return uint64(rawValue.Float()), nil
		}
	}
	return 0, fmt.Errorf("cannot convert %T to unsigned integer", value)
}

func historicalNumericFloat64(value any) (float64, error) {
	if raw, ok := value.([]byte); ok {
		return strconv.ParseFloat(string(raw), 64)
	}
	if text, ok := value.(string); ok {
		return strconv.ParseFloat(text, 64)
	}
	if number, ok := historicalNumericFloat(value); ok {
		return number, nil
	}
	return 0, fmt.Errorf("cannot convert %T to float", value)
}

func historicalNumericFloat(value any) (float64, bool) {
	rawValue := reflect.ValueOf(value)
	if !rawValue.IsValid() {
		return 0, false
	}
	switch rawValue.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(rawValue.Int()), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(rawValue.Uint()), true
	case reflect.Float32, reflect.Float64:
		return rawValue.Float(), true
	default:
		return 0, false
	}
}
