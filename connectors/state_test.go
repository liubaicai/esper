package connectors

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestStateManagerMatchesEsperAdapterTransitions(t *testing.T) {
	manager := NewStateManager()
	if got := manager.State(); got != Opened {
		t.Fatalf("initial state = %s, want %s", got, Opened)
	}
	if err := manager.Start(); err != nil {
		t.Fatal(err)
	}
	if err := manager.Pause(); err != nil {
		t.Fatal(err)
	}
	if err := manager.Resume(); err != nil {
		t.Fatal(err)
	}
	if err := manager.Stop(); err != nil {
		t.Fatal(err)
	}
	if err := manager.Start(); err != nil {
		t.Fatal(err)
	}
	if err := manager.Stop(); err != nil {
		t.Fatal(err)
	}
	if err := manager.Destroy(); err != nil {
		t.Fatal(err)
	}
	if got := manager.State(); got != Destroyed {
		t.Fatalf("final state = %s, want %s", got, Destroyed)
	}
}

func TestStateManagerRejectsInvalidTransitions(t *testing.T) {
	manager := NewStateManager()
	invalid := []struct {
		name string
		call func() error
	}{
		{"stop before start", manager.Stop},
		{"pause before start", manager.Pause},
		{"resume before start", manager.Resume},
	}
	for _, test := range invalid {
		t.Run(test.name, func(t *testing.T) {
			err := test.call()
			if !errors.Is(err, ErrInvalidTransition) {
				t.Fatalf("error = %v, want ErrInvalidTransition", err)
			}
		})
	}
	if err := manager.Start(); err != nil {
		t.Fatal(err)
	}
	if err := manager.Start(); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("repeated start error = %v", err)
	}
	if err := manager.Pause(); err != nil {
		t.Fatal(err)
	}
	if err := manager.Pause(); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("repeated pause error = %v", err)
	}
	if err := manager.Destroy(); err != nil {
		t.Fatal(err)
	}
	if err := manager.Resume(); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("resume after destroy error = %v", err)
	}
	if err := manager.Destroy(); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("repeated destroy error = %v", err)
	}
}

func TestStateManagerWaitsForStartAndReportsStop(t *testing.T) {
	manager := NewStateManager()
	result := make(chan error, 1)
	go func() { result <- manager.WaitUntilStarted(context.Background()) }()
	select {
	case err := <-result:
		t.Fatalf("wait returned before start: %v", err)
	case <-time.After(10 * time.Millisecond):
	}
	if err := manager.Start(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("wait after start: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("wait did not observe start")
	}
	if err := manager.Stop(); err != nil {
		t.Fatal(err)
	}
	if err := manager.WaitUntilStarted(context.Background()); !errors.Is(err, ErrStopped) {
		t.Fatalf("wait after stop = %v, want ErrStopped", err)
	}
}
