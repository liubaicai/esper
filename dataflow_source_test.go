package esper

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

type dataflowIntSource struct {
	values    []int
	lifecycle *[]string
}

func (s *dataflowIntSource) Open(context.Context) error {
	*s.lifecycle = append(*s.lifecycle, "open")
	return nil
}

func (s *dataflowIntSource) Close(context.Context) error {
	*s.lifecycle = append(*s.lifecycle, "close")
	return nil
}

func (s *dataflowIntSource) Run(ctx context.Context, emitter *DataflowEmitter) error {
	*s.lifecycle = append(*s.lifecycle, "run")
	for _, value := range s.values {
		if err := emitter.Submit(ctx, value); err != nil {
			return err
		}
	}
	return nil
}

type dataflowIntTransform struct{}

func (dataflowIntTransform) Process(_ context.Context, input DataflowInput) ([]DataflowEmission, error) {
	value, ok := input.Value.(int)
	if !ok {
		return nil, fmt.Errorf("expected int, got %T", input.Value)
	}
	return []DataflowEmission{Emit(value * 2)}, nil
}

func TestDataflowCustomSourceRunAndJoinMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	var lifecycle []string
	definition, err := DefineDataflow(env, "custom-source-flow").
		CustomTypedSource("source", func(DataflowOperatorContext) (DataflowSourceRuntime, error) {
			return &dataflowIntSource{values: []int{1, 2, 3}, lifecycle: &lifecycle}, nil
		}, []DataflowPort{DataflowPortOf[int]("out")}).
		CustomTypedPorts("double", func(DataflowOperatorContext) (DataflowOperatorRuntime, error) {
			return dataflowIntTransform{}, nil
		}, []DataflowPort{DataflowPortOf[int]("in")}, []DataflowPort{DataflowPortOf[int]("out")}).
		Emitter("sink").
		Connect("source", "double").
		Connect("double", "sink").
		Build()
	if err != nil {
		t.Fatal(err)
	}

	instance, err := NewEngine(env).InstantiateDataflow(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if instance.State() != DataflowComplete {
		t.Fatalf("custom source state = %v", instance.State())
	}
	if got, want := lifecycle, []string{"open", "run", "close"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("custom source lifecycle = %#v, want %#v", got, want)
	}
	if stats := instance.Stats(); stats.Processed != 3 || stats.Emitted != 3 || stats.Errors != 0 {
		t.Fatalf("custom source stats = %#v", stats)
	}
	outputs := instance.Outputs()
	if len(outputs) != 3 || outputs[0] != 2 || outputs[1] != 4 || outputs[2] != 6 {
		t.Fatalf("custom source outputs = %#v", outputs)
	}
}

type dataflowBlockingSource struct {
	started chan struct{}
	closed  chan struct{}
	release chan struct{}
}

func (s *dataflowBlockingSource) Open(context.Context) error {
	close(s.started)
	return nil
}

func (s *dataflowBlockingSource) Close(context.Context) error {
	select {
	case <-s.closed:
	default:
		close(s.closed)
	}
	return nil
}

func (s *dataflowBlockingSource) Run(ctx context.Context, emitter *DataflowEmitter) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.release:
		return emitter.Submit(ctx, 1)
	}
}

func TestDataflowCustomSourceCancelAndJoin(t *testing.T) {
	env := NewEnvironment()
	source := &dataflowBlockingSource{
		started: make(chan struct{}),
		closed:  make(chan struct{}),
		release: make(chan struct{}),
	}
	definition, err := DefineDataflow(env, "blocking-source-flow").
		CustomSource("source", func(DataflowOperatorContext) (DataflowSourceRuntime, error) {
			return source, nil
		}).
		Emitter("sink").
		Connect("source", "sink").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	instance, err := NewEngine(env).InstantiateDataflow(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-source.started:
	case <-time.After(2 * time.Second):
		t.Fatal("custom source did not open")
	}

	joined := make(chan error, 1)
	go func() { joined <- instance.Join(context.Background()) }()
	select {
	case err := <-joined:
		t.Fatalf("join returned before cancellation: %v", err)
	default:
	}
	if err := instance.Cancel(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := <-joined; !errors.Is(err, ErrorCanceled) {
		t.Fatalf("join cancellation error = %v", err)
	}
	if instance.State() != DataflowCanceled {
		t.Fatalf("canceled source state = %v", instance.State())
	}
	select {
	case <-source.closed:
	default:
		t.Fatal("source lifecycle was not closed")
	}
	if outputs := instance.Outputs(); len(outputs) != 0 {
		t.Fatalf("canceled source outputs = %#v", outputs)
	}
}

type dataflowFailingSource struct{ err error }

func (s dataflowFailingSource) Run(context.Context, *DataflowEmitter) error { return s.err }

func TestDataflowCustomSourceErrorPropagatesThroughRun(t *testing.T) {
	env := NewEnvironment()
	sentinel := errors.New("source failure")
	definition, err := DefineDataflow(env, "failing-source-flow").
		CustomSource("source", func(DataflowOperatorContext) (DataflowSourceRuntime, error) {
			return dataflowFailingSource{err: sentinel}, nil
		}).
		Emitter("sink").
		Connect("source", "sink").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	instance, err := NewEngine(env).InstantiateDataflow(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Run(context.Background()); !errors.Is(err, sentinel) {
		t.Fatalf("source run error = %v", err)
	}
	if instance.State() != DataflowComplete {
		t.Fatalf("failed source state = %v", instance.State())
	}
	if stats := instance.Stats(); stats.Errors != 1 || stats.Dropped != 0 {
		t.Fatalf("failed source stats = %#v", stats)
	}
	if last, ok := instance.LastError(); !ok || !errors.Is(last, sentinel) {
		t.Fatalf("failed source last error = %#v/%v", last, ok)
	}
}

type dataflowWrongTypedSource struct{}

func (dataflowWrongTypedSource) Run(ctx context.Context, emitter *DataflowEmitter) error {
	return emitter.Submit(ctx, "not-an-int")
}

func TestDataflowCustomSourceRejectsWrongOutputType(t *testing.T) {
	env := NewEnvironment()
	definition, err := DefineDataflow(env, "wrong-source-type-flow").
		CustomTypedSource("source", func(DataflowOperatorContext) (DataflowSourceRuntime, error) {
			return dataflowWrongTypedSource{}, nil
		}, []DataflowPort{DataflowPortOf[int]("out")}).
		Emitter("sink").
		Connect("source", "sink").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	instance, err := NewEngine(env).InstantiateDataflow(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	err = instance.Run(context.Background())
	if !errors.Is(err, ErrorTypeMismatch) {
		t.Fatalf("wrong source output error = %v, want TypeMismatch", err)
	}
	if instance.State() != DataflowComplete {
		t.Fatalf("wrong source output state = %v, want complete", instance.State())
	}
	if outputs := instance.Outputs(); len(outputs) != 0 {
		t.Fatalf("wrong source output values = %#v", outputs)
	}
}

func TestDataflowJoinRequiresStart(t *testing.T) {
	env := NewEnvironment()
	definition, err := DefineDataflow(env, "join-before-start").BeaconSource("source").Build()
	if err != nil {
		t.Fatal(err)
	}
	instance, err := NewEngine(env).InstantiateDataflow(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Join(context.Background()); !errors.Is(err, ErrorState) {
		t.Fatalf("join-before-start error = %v", err)
	}
}
