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

type dataflowRunnableSource struct {
	started chan struct{}
	release chan struct{}
	value   int
}

func (s *dataflowRunnableSource) Open(context.Context) error {
	close(s.started)
	return nil
}

func (s *dataflowRunnableSource) Close(context.Context) error { return nil }

func (s *dataflowRunnableSource) Run(ctx context.Context, emitter *DataflowEmitter) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.release:
		return emitter.Submit(ctx, s.value)
	}
}

func TestDataflowMultipleCustomSourcesJoinAfterEachCompletesMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	first := &dataflowRunnableSource{started: make(chan struct{}), release: make(chan struct{}), value: 1}
	second := &dataflowRunnableSource{started: make(chan struct{}), release: make(chan struct{}), value: 2}
	definition, err := DefineDataflow(env, "multiple-source-flow").
		CustomTypedSource("source-one", func(DataflowOperatorContext) (DataflowSourceRuntime, error) {
			return first, nil
		}, []DataflowPort{DataflowPortOf[int]("out")}).
		CustomTypedSource("source-two", func(DataflowOperatorContext) (DataflowSourceRuntime, error) {
			return second, nil
		}, []DataflowPort{DataflowPortOf[int]("out")}).
		Emitter("sink").
		Connect("source-one", "sink").
		Connect("source-two", "sink").
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
	for name, started := range map[string]<-chan struct{}{"source-one": first.started, "source-two": second.started} {
		select {
		case <-started:
		case <-time.After(2 * time.Second):
			t.Fatalf("%s did not start, state=%v stats=%#v", name, instance.State(), instance.Stats())
		}
	}
	if instance.State() != DataflowRunning {
		t.Fatalf("multiple-source state = %v, want running", instance.State())
	}

	close(first.release)
	deadline := time.After(2 * time.Second)
	for len(instance.Outputs()) < 1 {
		select {
		case <-deadline:
			t.Fatal("first source did not reach sink")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	if instance.State() != DataflowRunning {
		t.Fatalf("multiple-source flow completed after one source, state = %v", instance.State())
	}
	close(second.release)
	if err := instance.Join(context.Background()); err != nil {
		t.Fatal(err)
	}
	outputs := instance.Outputs()
	if len(outputs) != 2 || outputs[0] != 1 || outputs[1] != 2 {
		t.Fatalf("multiple-source outputs = %#v, want [1 2]", outputs)
	}
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

func TestDataflowSourceExceptionContextMatchesEsper(t *testing.T) {
	env := NewEnvironment()
	sentinel := errors.New("My-Exception-Is-Here")
	definition, err := DefineDataflow(env, "source-exception-flow").
		CustomSource("source", func(DataflowOperatorContext) (DataflowSourceRuntime, error) {
			return dataflowFailingSource{err: sentinel}, nil
		}).
		Emitter("sink").
		Connect("source", "sink").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	var observed DataflowError
	instance, err := NewEngine(env).InstantiateDataflowWithOptions(context.Background(), definition, DataflowOptions{
		ExceptionHandler: func(_ context.Context, failure DataflowError) error {
			observed = failure
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Run(context.Background()); !errors.Is(err, sentinel) {
		t.Fatalf("source exception run error = %v", err)
	}
	if instance.State() != DataflowComplete {
		t.Fatalf("source exception state = %v, want complete", instance.State())
	}
	if observed.OperatorName != "source" || observed.OperatorNum != 0 || observed.OperatorPrettyPrint != "source#0() -> out" || !errors.Is(observed, sentinel) {
		t.Fatalf("source exception context = %#v", observed)
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

type dataflowNamedSource struct{}

func (dataflowNamedSource) Run(ctx context.Context, emitter *DataflowEmitter) error {
	if err := emitter.SubmitPort(ctx, "text", "left"); err != nil {
		return err
	}
	return emitter.SubmitPort(ctx, "number", 7)
}

type dataflowNamedSourceRuntime struct{}

func (dataflowNamedSourceRuntime) Process(_ context.Context, input DataflowInput) ([]DataflowEmission, error) {
	switch value := input.Value.(type) {
	case string:
		return []DataflowEmission{Emit("text:" + value)}, nil
	case int:
		return []DataflowEmission{Emit(fmt.Sprintf("number:%d", value))}, nil
	default:
		return nil, fmt.Errorf("unexpected named source value %T", input.Value)
	}
}

func TestDataflowCustomSourceNamedPortsRouteAndTypeCheck(t *testing.T) {
	env := NewEnvironment()
	definition, err := DefineDataflow(env, "named-source-flow").
		CustomTypedSource("source", func(DataflowOperatorContext) (DataflowSourceRuntime, error) {
			return dataflowNamedSource{}, nil
		}, []DataflowPort{DataflowPortOf[string]("text"), DataflowPortOf[int]("number")}).
		CustomTypedPorts("format", func(DataflowOperatorContext) (DataflowOperatorRuntime, error) {
			return dataflowNamedSourceRuntime{}, nil
		}, []DataflowPort{DataflowPortOf[string]("text"), DataflowPortOf[int]("number")}, []DataflowPort{DataflowPortOf[string]("out")}).
		Emitter("sink").
		ConnectPorts("source", "text", "format", "text").
		ConnectPorts("source", "number", "format", "number").
		Connect("format", "sink").
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
	if got := instance.Outputs(); len(got) != 2 || got[0] != "text:left" || got[1] != "number:7" {
		t.Fatalf("named source outputs = %#v", got)
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
