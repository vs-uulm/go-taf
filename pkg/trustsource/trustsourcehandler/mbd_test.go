package trustsourcehandler

import (
	"testing"

	"github.com/vs-uulm/go-taf/pkg/core"
	mbdmsg "github.com/vs-uulm/go-taf/pkg/message/mbd"
)

func TestAddMbdDoubleEvidence(t *testing.T) {
	observation := mbdmsg.ObservationSet{
		RelativePositionErrorX:   floatPointer(0.1),
		RelativePositionErrorY:   floatPointer(0.2),
		SenderSpeedErrorX:        floatPointer(0.3),
		SenderSpeedErrorY:        floatPointer(0.4),
		SenderAccelerationErrorX: floatPointer(0.5),
		SenderAccelerationErrorY: floatPointer(0.6),
		DistanceToRoadEdgeError:  floatPointer(0.7),
		ReceiverTimeError:        floatPointer(0.8),
		SenderHeadingErrorSin:    floatPointer(0.9),
		SenderHeadingErrorCos:    floatPointer(1.0),
	}

	evidence := make(map[core.EvidenceType]interface{})
	addMbdDoubleEvidence(evidence, observation)

	want := map[core.EvidenceType]float64{
		core.MBD_RELATIVE_POSITION_ERROR_X:   0.1,
		core.MBD_RELATIVE_POSITION_ERROR_Y:   0.2,
		core.MBD_SENDER_SPEED_ERROR_X:        0.3,
		core.MBD_SENDER_SPEED_ERROR_Y:        0.4,
		core.MBD_SENDER_ACCELERATION_ERROR_X: 0.5,
		core.MBD_SENDER_ACCELERATION_ERROR_Y: 0.6,
		core.MBD_DISTANCE_TO_ROAD_EDGE_ERROR: 0.7,
		core.MBD_RECEIVER_TIME_ERROR:         0.8,
		core.MBD_SENDER_HEADING_ERROR_SIN:    0.9,
		core.MBD_SENDER_HEADING_ERROR_COS:    1.0,
	}

	for evidenceType, value := range want {
		got, ok := evidence[evidenceType]
		if !ok {
			t.Fatalf("missing evidence %s", evidenceType)
		}
		if got != value {
			t.Fatalf("evidence %s = %v, want %v", evidenceType, got, value)
		}
	}
}

func TestHasRequiredMbdEvidenceIgnoresOtherTrustSources(t *testing.T) {
	evidence := map[core.EvidenceType]interface{}{
		core.MBD_MISBEHAVIOR_REPORT: 0,
	}
	required := []core.EvidenceType{
		core.MBD_MISBEHAVIOR_REPORT,
		core.NTM_REMOTE_OPINION,
	}

	if !hasRequiredMbdEvidence(evidence, required) {
		t.Fatal("expected available MBD evidence to satisfy MBD requirements")
	}
}

func floatPointer(value float64) *float64 {
	return &value
}
