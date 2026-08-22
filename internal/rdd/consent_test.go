package rdd

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestConsentResult_JSONRoundTrip(t *testing.T) {
	want := ConsentResult{
		Headline: "Consent required before freezing candidate",
		Reason:   "review start requires explicit consent",
		Evidence: "candidate hash abc123",
		Granted:  true,
	}

	data, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("json.Marshal(%+v) returned unexpected error: %v", want, err)
	}

	var got ConsentResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal(%q) returned unexpected error: %v", data, err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, want)
	}
}

func TestParseConsentFlag(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    ConsentFlag
		wantErr bool
	}{
		{name: "relay is valid", input: "relay", want: ConsentRelay},
		{name: "granted is valid", input: "granted", want: ConsentGranted},
		{name: "declined is valid", input: "declined", want: ConsentDeclined},
		{name: "empty string is invalid", input: "", wantErr: true},
		{name: "unrecognized value is invalid", input: "yes", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseConsentFlag(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseConsentFlag(%q) expected an error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseConsentFlag(%q) returned unexpected error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("ParseConsentFlag(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
