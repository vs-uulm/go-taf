package validator

import (
	"testing"

	"github.com/vs-uulm/go-taf/pkg/message"
)

const mbdNotifyPrefix = `{
  "subscriptionId": "sub-1",
  "CPM_REPORT": {
    "generationTime": 1,
    "reporterPseudoId": 2,
    "reporterArteryId": 3,
    "version": 1,
    "observationlocation": {"x": 1, "y": 2},
    "content": {
      "observationSet": [`

const mbdNotifySuffix = `],
      "V2XPduEvidence": {"sourceId": 1, "referenceTime": 2}
    }
  }
}`

const mbdDoubleObservation = `{
  "targetId": 2,
  "relative_position_error_x": 0.1,
  "relative_position_error_y": 0.2,
  "sender_speed_error_x": 0.3,
  "sender_speed_error_y": 0.4,
  "sender_acceleration_error_x": 0.5,
  "sender_acceleration_error_y": 0.6,
  "distance_to_road_edge_error": 0.7,
  "receiver_time_error": 0.8,
  "sender_heading_error_sin": 0.9,
  "sender_heading_error_cos": 1.0
}`

func TestValidateMBDNotifyEvidenceFormats(t *testing.T) {
	tests := []struct {
		name        string
		observation string
		wantValid   bool
	}{
		{
			name:        "legacy check evidence",
			observation: `{"targetId": 2, "check": 0}`,
			wantValid:   true,
		},
		{
			name:        "double evidence",
			observation: mbdDoubleObservation,
			wantValid:   true,
		},
		{
			name:        "mixed evidence",
			observation: `{"targetId": 2, "check": 0, "relative_position_error_x": 0.1}`,
			wantValid:   false,
		},
		{
			name: "incomplete double evidence",
			observation: `{
  "targetId": 2,
  "relative_position_error_x": 0.1,
  "relative_position_error_y": 0.2,
  "sender_speed_error_x": 0.3,
  "sender_speed_error_y": 0.4,
  "sender_acceleration_error_x": 0.5,
  "sender_acceleration_error_y": 0.6,
  "distance_to_road_edge_error": 0.7,
  "receiver_time_error": 0.8,
  "sender_heading_error_sin": 0.9
}`,
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, errs, err := Validate(message.MBD_NOTIFY, mbdNotifyPrefix+tt.observation+mbdNotifySuffix)
			if err != nil {
				t.Fatalf("Validate returned error: %v", err)
			}
			if valid != tt.wantValid {
				t.Fatalf("valid = %t, want %t, validation errors: %v", valid, tt.wantValid, errs)
			}
		})
	}
}
