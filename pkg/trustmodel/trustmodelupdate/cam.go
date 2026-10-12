package trustmodelupdate

import (
	"encoding/json"
	"github.com/vs-uulm/go-taf/pkg/core"
	v2xmsg "github.com/vs-uulm/go-taf/pkg/message/v2x"
)

type RefreshCAM struct {
	sourceID      string
	opinion       v2xmsg.CamPositionOpinion
	referenceTime int64
}

func (r RefreshCAM) SourceID() string {
	return r.sourceID
}

func (r RefreshCAM) Opinion() v2xmsg.CamPositionOpinion {
	return r.opinion
}

func (r RefreshCAM) ReferenceTime() int64 {
	return r.referenceTime
}

func CreateRefreshCAM(sourceID string, opinion v2xmsg.CamPositionOpinion, refTime int64) RefreshCAM {
	return RefreshCAM{
		sourceID:      sourceID,
		opinion:       opinion,
		referenceTime: refTime,
	}
}

func (r RefreshCAM) Type() core.UpdateOp {
	return core.REFRESH_CAM
}

/*
MarshalJSON leaves out the reference time, as the time of each update is shown separately (see package clock); it is
still available via ReferenceTime.
*/
func (r RefreshCAM) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		SourceID string                    `json:"sourceID"`
		Opinion  v2xmsg.CamPositionOpinion `json:"opinion"`
		Update   string                    `json:"update"`
	}{
		SourceID: r.SourceID(),
		Opinion:  r.opinion,
		Update:   r.Type().String(),
	})
}
