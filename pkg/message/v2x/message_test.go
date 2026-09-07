package v2xmsg

import "testing"

func TestUnmarshalV2XCam(t *testing.T) {
	cam, err := UnmarshalV2XCam([]byte(`{
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
	}`))
	if err != nil {
		t.Fatalf("UnmarshalV2XCam returned an error: %v", err)
	}
	if cam.Opinions.Position.BaseRate != 0.5 {
		t.Fatalf("unexpected base rate: %v", cam.Opinions.Position.BaseRate)
	}
}
