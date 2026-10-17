package variables

import (
	"errors"
	"testing"
)

func TestExpandContract(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		values map[string]string
		want   string
	}{
		{name: "value", input: "look at ${target}", values: map[string]string{"target": "bronze lamp"}, want: "look at bronze lamp"},
		{name: "fallback missing", input: "count ${count:25}", want: "count 25"},
		{name: "fallback empty", input: "count ${count:25}", values: map[string]string{"count": ""}, want: "count 25"},
		{name: "empty without fallback", input: "[${message}]", values: map[string]string{"message": ""}, want: "[]"},
		{name: "literal reference", input: `say \${target}`, values: map[string]string{"target": "Marcus"}, want: "say ${target}"},
		{name: "literal directive", input: `say \$(wait 2)`, want: "say $(wait 2)"},
		{name: "one pass", input: "say ${message}", values: map[string]string{"message": "${target}", "target": "Marcus"}, want: "say ${target}"},
		{name: "unicode", input: "say ${message}", values: map[string]string{"message": "salvē, Rōma"}, want: "say salvē, Rōma"},
		{name: "data stays data", input: "say ${message}&&look", values: map[string]string{"message": "one;;two$(notify no)"}, want: "say one;;two$(notify no)&&look"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := Expand(test.input, test.values)
			if err != nil {
				t.Fatalf("Expand: %v", err)
			}
			if got != test.want {
				t.Fatalf("Expand = %q, want %q", got, test.want)
			}
		})
	}
}

func TestExpandReturnsStructuredErrors(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantCode ErrorCode
		wantName string
		wantByte int
	}{
		{name: "unresolved", input: "say ${target}", wantCode: ErrorUnresolved, wantName: "target", wantByte: 4},
		{name: "invalid name", input: "say ${bad-name}", wantCode: ErrorInvalidName, wantName: "bad-name", wantByte: 4},
		{name: "unterminated", input: "say ${target", wantCode: ErrorUnterminated, wantByte: 4},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Expand(test.input, nil)
			var expansionErr *ExpansionError
			if !errors.As(err, &expansionErr) {
				t.Fatalf("Expand error = %T %v, want *ExpansionError", err, err)
			}
			if expansionErr.Code != test.wantCode || expansionErr.Name != test.wantName || expansionErr.ByteOffset != test.wantByte {
				t.Fatalf("ExpansionError = %+v, want code=%q name=%q byte=%d", expansionErr, test.wantCode, test.wantName, test.wantByte)
			}
		})
	}
}

func TestValidName(t *testing.T) {
	for _, name := range []string{"target", "target_2", "_private", "A"} {
		if !ValidName(name) {
			t.Errorf("ValidName(%q) = false", name)
		}
	}
	for _, name := range []string{"", "2target", "bad-name", "naïve", "with space"} {
		if ValidName(name) {
			t.Errorf("ValidName(%q) = true", name)
		}
	}
}
