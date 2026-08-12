package esper

import (
	"context"
	"fmt"
)

// TableSink persists aggregate/projection rows into an Engine table. It is a
// Go-native equivalent of an into-table result consumer: every new-stream Row
// is validated and atomically upserted according to the table's primary key.
// Old-stream rows are intentionally ignored because table state represents
// the latest value for each key rather than a second append-only stream.
type TableSink struct {
	table *Table
}

// NewTableSink binds a result sink to an already registered table. Binding at
// construction time keeps deployment errors deterministic and avoids looking
// up a mutable table name for every result batch.
func NewTableSink(engine *Engine, tableName string) (*TableSink, error) {
	if engine == nil {
		return nil, NewError(ErrorDependency, "table sink requires an engine")
	}
	table, ok := engine.Table(tableName)
	if !ok {
		return nil, NewError(ErrorUnknownName, "table sink table does not exist: "+tableName)
	}
	return &TableSink{table: table}, nil
}

func (s *TableSink) Write(ctx context.Context, batch ResultBatch) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	if s == nil || s.table == nil {
		return NewError(ErrorDependency, "table sink is not initialized")
	}
	for index, result := range batch.New {
		if err := contextErr(ctx); err != nil {
			return err
		}
		row, ok := result.Row()
		if !ok {
			return NewError(ErrorTypeMismatch, "table sink requires Row results")
		}
		if _, err := s.table.Upsert(ctx, row.AsMap()); err != nil {
			return WrapError(ErrorState, fmt.Sprintf("table sink row %d", index), err)
		}
	}
	return nil
}

func (s *TableSink) Close() error { return nil }
