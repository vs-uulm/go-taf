package validator

import (
	"testing"

	"github.com/vs-uulm/go-taf/pkg/message"
)

func TestValidateGenericRequestAwaitSettlement(t *testing.T) {
	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{
			name:  "enabled",
			input: `{"sender":"simulator","serviceType":"TAS","messageType":"TAS_TA_REQUEST","responseTopic":"application","requestId":"request-1","awaitSettlement":true,"message":{}}`,
			valid: true,
		},
		{
			name:  "wrong type",
			input: `{"sender":"simulator","serviceType":"TAS","messageType":"TAS_TA_REQUEST","responseTopic":"application","requestId":"request-1","awaitSettlement":"true","message":{}}`,
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, errs, err := Validate(message.GENERIC_REQUEST, tt.input)
			if err != nil {
				t.Fatalf("Validate returned error: %v", err)
			}
			if valid != tt.valid {
				t.Fatalf("valid = %t, want %t, validation errors: %v", valid, tt.valid, errs)
			}
		})
	}
}
