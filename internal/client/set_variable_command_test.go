package client

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/cyber-godzilla/praetor/internal/types"
)

func TestSetVariableCommandSavesUpdatesAndEmits(t *testing.T) {
	c := newDiscTestClient(t)
	defer c.Engine.Close()

	var saved map[string]string
	c.SetInputVariableSaver(func(variables map[string]string) error {
		saved = cloneVariables(variables)
		return nil
	})

	if err := c.SendInput(`/set-var speaker "A man in a hooded cloak"`); err != nil {
		t.Fatalf("SendInput: %v", err)
	}
	want := map[string]string{"speaker": "A man in a hooded cloak"}
	if got := c.inputVariableSnapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("live variables = %#v, want %#v", got, want)
	}
	if !reflect.DeepEqual(saved, want) {
		t.Fatalf("saved variables = %#v, want %#v", saved, want)
	}
	expanded, err := c.ExpandInputVariables("greet ${speaker}")
	if err != nil || expanded != "greet A man in a hooded cloak" {
		t.Fatalf("expanded = %q, %v", expanded, err)
	}

	event := <-c.Events()
	changed, ok := event.(types.InputVariableSetEvent)
	if !ok || changed.Name != "speaker" || changed.Value != "A man in a hooded cloak" {
		t.Fatalf("event = %#v, want speaker variable change", event)
	}

	if err := c.SendInput(`/set-variable speaker ""`); err != nil {
		t.Fatalf("empty value: %v", err)
	}
	if got := c.inputVariableSnapshot()["speaker"]; got != "" {
		t.Fatalf("empty value stored as %q", got)
	}
}

func TestPraetorScriptVariableDirectivesPersistAndCompose(t *testing.T) {
	c := newDiscTestClient(t)
	defer c.Engine.Close()
	c.SetSemicolonDelay(10 * time.Millisecond)

	var saved map[string]string
	c.SetInputVariableSaver(func(variables map[string]string) error {
		saved = cloneVariables(variables)
		return nil
	})

	if err := c.SendInput(`$(set-var counter 2);;$(add-var counter 3.5)`); err != nil {
		t.Fatalf("SendInput: %v", err)
	}
	deadline := time.Now().Add(time.Second)
	for c.inputVariableSnapshot()["counter"] != "5.5" && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	want := map[string]string{"counter": "5.5"}
	if got := c.inputVariableSnapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("live variables = %#v, want %#v", got, want)
	}
	if !reflect.DeepEqual(saved, want) {
		t.Fatalf("saved variables = %#v, want %#v", saved, want)
	}
	waitForInputChainInactive(t, c)

	for _, value := range []string{"2", "5.5"} {
		event := <-c.Events()
		changed, ok := event.(types.InputVariableSetEvent)
		if !ok || changed.Name != "counter" || changed.Value != value {
			t.Fatalf("event = %#v, want counter=%s variable change", event, value)
		}
	}
}

func TestAddVariableDirectiveRejectsMissingNonNumericAndFailedSave(t *testing.T) {
	c := newDiscTestClient(t)
	defer c.Engine.Close()
	c.SetInputVariables(map[string]string{"words": "many", "counter": "4"})

	for _, input := range []string{`$(add-var missing 1)`, `$(add-var words 1)`} {
		if err := c.SendInput(input); err == nil {
			t.Fatalf("SendInput(%q) succeeded, want error", input)
		}
	}
	c.SetInputVariableSaver(func(map[string]string) error { return errors.New("disk full") })
	if err := c.SendInput(`$(add-var counter 2)`); err == nil {
		t.Fatal("persistence failure was not returned")
	}
	if got := c.inputVariableSnapshot()["counter"]; got != "4" {
		t.Fatalf("failed add stored %q, want original value 4", got)
	}
}

func TestSetVariableCommandRejectsInvalidNameAndPersistenceFailure(t *testing.T) {
	c := newDiscTestClient(t)
	defer c.Engine.Close()

	called := false
	c.SetInputVariableSaver(func(map[string]string) error {
		called = true
		return errors.New("disk full")
	})
	if err := c.SendInput(`/set-variable bad-name value`); err == nil {
		t.Fatal("invalid variable name was accepted")
	}
	if called {
		t.Fatal("persistence hook called for invalid name")
	}
	if err := c.SendInput(`/set-variable speaker Apula`); err == nil {
		t.Fatal("persistence failure was not returned")
	}
	if _, ok := c.inputVariableSnapshot()["speaker"]; ok {
		t.Fatal("failed save changed the live variable snapshot")
	}
}
