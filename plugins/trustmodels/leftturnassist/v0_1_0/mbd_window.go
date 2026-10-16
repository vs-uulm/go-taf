package leftturnassist_v0_1_0

// Windowing of the MBD opinions of a vehicle (one per received CAM).

import (
	"fmt"
	"strconv"

	"github.com/vs-uulm/go-subjectivelogic/pkg/subjectivelogic"
)

type mbdWindowWeighting string

const (
	mbdWindowEqual    mbdWindowWeighting = "equal"    // mean of the last MBD_WINDOW_SIZE opinions
	mbdWindowLinear   mbdWindowWeighting = "linear"   // weighted mean, weights 1..n from oldest to newest
	mbdWindowTemporal mbdWindowWeighting = "temporal" // Temporal-ATL, Bassi et al., VTC2026-Spring, eq. (1)
)

// mbdWindowing combines the recent MBD opinions into the opinion V_ego -> C_x_x
type mbdWindowing struct {
	weighting mbdWindowWeighting
	size      int
	gamma     float64
	tau       float64

	window   []subjectivelogic.QueryableOpinion
	temporal subjectivelogic.QueryableOpinion
}

// newMbdWindowing reads MBD_WINDOW_WEIGHTING, MBD_WINDOW_SIZE, MBD_TEMPORAL_GAMMA and MBD_TEMPORAL_TAU (see OnNewVehicle)
func newMbdWindowing(params map[string]string) mbdWindowing {
	w := mbdWindowing{weighting: mbdWindowEqual, size: 1}
	if value, exists := params["MBD_WINDOW_WEIGHTING"]; exists {
		switch mbdWindowWeighting(value) {
		case mbdWindowEqual, mbdWindowLinear, mbdWindowTemporal:
			w.weighting = mbdWindowWeighting(value)
		default:
			panic("Invalid MBD_WINDOW_WEIGHTING set: " + value)
		}
	}
	if value, exists := params["MBD_WINDOW_SIZE"]; exists {
		if size, err := strconv.Atoi(value); err == nil && size >= 1 {
			w.size = size
		} else {
			panic("Could not parse MBD_WINDOW_SIZE")
		}
	}
	if w.weighting == mbdWindowTemporal {
		w.gamma = mbdUnitParam(params, "MBD_TEMPORAL_GAMMA")
		w.tau = mbdUnitParam(params, "MBD_TEMPORAL_TAU")
	}
	return w
}

// mbdUnitParam parses a mandatory parameter in [0,1]
func mbdUnitParam(params map[string]string, name string) float64 {
	value, exists := params[name]
	if !exists {
		panic("Missing value: " + name)
	}
	number, err := strconv.ParseFloat(value, 64)
	if err != nil || number < 0 || number > 1 {
		panic(fmt.Sprintf("Could not parse %s", name))
	}
	return number
}

func (w *mbdWindowing) reset() {
	w.window = nil
	w.temporal = nil
}

// add adds the newest MBD opinion and returns the combined opinion
func (w *mbdWindowing) add(opinion subjectivelogic.QueryableOpinion) subjectivelogic.QueryableOpinion {
	if w.weighting == mbdWindowTemporal {
		return w.addTemporal(opinion)
	}
	if w.size <= 1 {
		return opinion
	}
	w.window = append(w.window, opinion)
	if len(w.window) > w.size {
		w.window = w.window[len(w.window)-w.size:]
	}
	//mean per component; a fusion operator would let a nearly dogmatic opinion dominate the window
	var belief, disbelief, uncertainty, baseRate, total float64
	for i, o := range w.window {
		weight := 1.0
		if w.weighting == mbdWindowLinear {
			weight = float64(i + 1)
		}
		belief += weight * o.Belief()
		disbelief += weight * o.Disbelief()
		uncertainty += weight * o.Uncertainty()
		baseRate += weight * o.BaseRate()
		total += weight
	}
	mean, err := subjectivelogic.NewOpinion(belief/total, disbelief/total, uncertainty/total, baseRate/total)
	if err != nil {
		return opinion
	}
	return &mean
}

// addTemporal: b_t = b_{t-1} - gamma*(tau - b_new) if in [0,1], else b_{t-1}; u_t = u_new; d_t = 1 - b_t - u_t
func (w *mbdWindowing) addTemporal(opinion subjectivelogic.QueryableOpinion) subjectivelogic.QueryableOpinion {
	//start with the first opinion that is not full uncertainty
	if w.temporal == nil {
		if opinion.Uncertainty() < 1 {
			w.temporal = opinion
		}
		return opinion
	}
	belief := w.temporal.Belief()
	if candidate := belief - w.gamma*(w.tau-opinion.Belief()); candidate >= 0 && candidate <= 1 {
		belief = candidate
	}
	uncertainty := opinion.Uncertainty()
	//not covered by the paper: keep b_t + u_t <= 1
	belief = min(belief, 1-uncertainty)
	temporal, err := subjectivelogic.NewOpinion(belief, max(0, 1-belief-uncertainty), uncertainty, opinion.BaseRate())
	if err != nil {
		return opinion
	}
	w.temporal = &temporal
	return &temporal
}
