package validator

import (
	"testing"

	"github.com/vs-uulm/go-taf/pkg/message"
)

func TestValidateV2XCam(t *testing.T) {
	valid, errs, err := Validate(message.V2X_CAM, `{
		"referenceTime": 22901900000,
		"sourceId": 0,
		"latitude": 48.3984,
		"longitude": 9.9916,
		"opinions": {
			"position": {
				"belief": 0.9896,
				"disbelief": 0.0,
				"uncertainty": 0.0104,
				"baserate": 0.5
			}
		}
	}`)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if !valid {
		t.Fatalf("expected V2X_CAM payload to be valid: %v", errs)
	}
}
