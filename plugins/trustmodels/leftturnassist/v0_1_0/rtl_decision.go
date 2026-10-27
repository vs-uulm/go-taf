package leftturnassist_v0_1_0

// RTL-based trust deciders: risk-based, Random Forest and Optuna.
// Each one supports the component rule or the projected-probability rule (PUT/PTT).
// Trained models are selected by the TAF configuration: <TRUST_DECISION_MODEL_DIR>/<TD>-<TF>-<TQM>[-<window>]/<file>.
// A missing or invalid parameter and a trained model that cannot be read (no model selectable, missing or corrupt
// file, invalid values) make the TMI panic, like the other parameters of the trust model.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/vs-uulm/go-subjectivelogic/pkg/subjectivelogic"
	"github.com/vs-uulm/go-taf/pkg/core"

	"github.com/vs-uulm/go-taf/plugins/trustmodels/leftturnassist/v0_1_0/rtl"
)

// Trained models: one folder per TAF configuration, named as in eval/results-matrix.sh (e.g. BR-ACF-bspline).
const (
	defaultRTLModelDir = "/opt/rtl-models" // rtl-models/ of the evaluation repo, mounted by run-simulation.sh
	rfModelFile        = "rf_model.json"
	optunaModelFile    = "rtl_optuna.json"
)

// Short names of the operators, as in eval/results-matrix.sh. A name not in these maps is used as is, as in the
// results matrix.
var (
	tdShortNames  = map[string]string{"Base_Rate_Sensitive": "BR", "Uncertainty_Favouring": "UF", "Disbelief_Favouring": "DF"}
	tfShortNames  = map[string]string{"Aleatory_Cumulative": "ACF", "Epistemic_Cumulative": "ECF", "Weighted": "WBF", "Consensus_Compromise": "CCF", "Proportional": "PF"}
	tqmShortNames = map[string]string{"f1-optimized": "analytic_f1", "analytic": "analytic", "b-spline": "bspline", "mlp": "mlp"}
)

// shortName maps a parameter value to its short name; unset uses unset, unknown names stay as they are.
func shortName(names map[string]string, value, unset string) string {
	if value == "" {
		return unset
	}
	if short, ok := names[value]; ok {
		return short
	}
	return value
}

// rtlConfigName returns the TAF configuration <TD>-<TF>-<TQM>[-<window>], e.g. BR-ACF-bspline or
// BR-ACF-bspline-temporal-g0.2-t0.5. Unset operators use the TAF defaults (BR, ACF), as in OnNewVehicle.
func rtlConfigName(params map[string]string) (string, error) {
	tqm := params["TRUST_QUANTIFICATION_MECHANISM_MBD"]
	if tqm == "" {
		return "", errors.New("cannot select the model: TRUST_QUANTIFICATION_MECHANISM_MBD is not set")
	}
	name := shortName(tdShortNames, params["DISCOUNTING_OPERATOR"], "BR") + "-" +
		shortName(tfShortNames, params["FUSION_OPERATOR"], "ACF") + "-" +
		shortName(tqmShortNames, tqm, "")
	if window := rtlWindowName(newMbdWindowing(params)); window != "off" {
		name += "-" + window
	}
	return name, nil
}

// rtlWindowName names the MBD windowing as the window column of eval/results-matrix.sh:
// off, <equal|linear>-n<size> or temporal-g<gamma>-t<tau>.
func rtlWindowName(w mbdWindowing) string {
	number := func(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }
	switch {
	case w.weighting == mbdWindowTemporal:
		return "temporal-g" + number(w.gamma) + "-t" + number(w.tau)
	case w.size > 1:
		return string(w.weighting) + "-n" + strconv.Itoa(w.size)
	}
	return "off"
}

// modelPath returns the explicit path (pathKey) if set, else the model of the current configuration.
func modelPath(params map[string]string, pathKey, file string) (string, error) {
	if path := params[pathKey]; path != "" {
		return path, nil
	}
	config, err := rtlConfigName(params)
	if err != nil {
		return "", err
	}
	dir := params["TRUST_DECISION_MODEL_DIR"]
	if dir == "" {
		dir = defaultRTLModelDir
	}
	return filepath.Join(dir, config, file), nil
}

// trustDeciderFunc is the signature used by TrustModelInstance.trustDecision.
type trustDeciderFunc = func(proposition string, atl subjectivelogic.QueryableOpinion, rtl subjectivelogic.QueryableOpinion) core.TrustDecision

// Model cache: each file is read once per path (result or error), not once per vehicle.
var rtlCache = struct {
	sync.Mutex
	rf     map[string]*rtl.Model
	optuna map[string]rtl.RTL
	errs   map[string]error
}{rf: map[string]*rtl.Model{}, optuna: map[string]rtl.RTL{}, errs: map[string]error{}}

// logLoaded logs once which model file is used.
func logLoaded(kind, path string) {
	slog.Default().Info("RTL model loaded", "TRUST_DECISION", kind, "file", path)
}

// thresholdParam reads a mandatory threshold in [0,1]; it panics if the parameter is missing or invalid.
func thresholdParam(params map[string]string, key string) float64 {
	raw, exists := params[key]
	if !exists {
		panic("Missing value: " + key)
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || !(value >= 0 && value <= 1) {
		panic("Could not parse " + key)
	}
	return value
}

// checkFile fails if the trained model file does not exist.
func checkFile(path, key, trainCmd string) error {
	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%s file '%s' not found (train first: %s)", key, path, trainCmd)
	}
	if err != nil {
		return fmt.Errorf("could not access %s file '%s': %w", key, path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("%s '%s' is a directory", key, path)
	}
	return nil
}

// loadRFModel loads (once) the Random Forest model.
func loadRFModel(path string) (*rtl.Model, error) {
	rtlCache.Lock()
	defer rtlCache.Unlock()
	if m, ok := rtlCache.rf[path]; ok {
		return m, nil
	}
	if err, ok := rtlCache.errs["rf:"+path]; ok {
		return nil, err
	}
	m, err := func() (*rtl.Model, error) {
		if err := checkFile(path, "RF model", "eval/train-rtl.sh --method rf"); err != nil {
			return nil, err
		}
		m, err := rtl.LoadModel(path)
		if err != nil {
			return nil, fmt.Errorf("corrupt RF model '%s': %w", path, err)
		}
		return m, nil
	}()
	if err != nil {
		rtlCache.errs["rf:"+path] = err
		return nil, err
	}
	rtlCache.rf[path] = m
	logLoaded("RTL_RandomForest", path)
	return m, nil
}

// loadOptunaRTL loads (once) the optimal RTL from rtl_optuna.json.
func loadOptunaRTL(path string) (rtl.RTL, error) {
	rtlCache.Lock()
	defer rtlCache.Unlock()
	if r, ok := rtlCache.optuna[path]; ok {
		return r, nil
	}
	if err, ok := rtlCache.errs["optuna:"+path]; ok {
		return rtl.RTL{}, err
	}
	r, err := func() (rtl.RTL, error) {
		if err := checkFile(path, "Optuna model", "eval/train-rtl.sh --method optuna"); err != nil {
			return rtl.RTL{}, err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return rtl.RTL{}, fmt.Errorf("could not read Optuna model '%s': %w", path, err)
		}
		var file struct {
			RTL *rtl.RTL `json:"rtl"`
		}
		if err := json.Unmarshal(data, &file); err != nil {
			return rtl.RTL{}, fmt.Errorf("corrupt Optuna model '%s': %w", path, err)
		}
		if file.RTL == nil {
			return rtl.RTL{}, fmt.Errorf("Optuna model '%s' has no \"rtl\" field", path)
		}
		if err := file.RTL.Validate(); err != nil {
			return rtl.RTL{}, fmt.Errorf("Optuna model '%s': %w", path, err)
		}
		return *file.RTL, nil
	}()
	if err != nil {
		rtlCache.errs["optuna:"+path] = err
		return rtl.RTL{}, err
	}
	rtlCache.optuna[path] = r
	logLoaded("RTL_Optuna", path)
	return r, nil
}

// newRTLTrustDecider builds the decider for TRUST_DECISION = RTL_RiskBased | RTL_RandomForest | RTL_Optuna.
//
//	TRUST_DECISION_LOGIC: Components (default) | ProjectedProbability
//	RTL_RiskBased    -> TRUST_DECISION_BT, TRUST_DECISION_DT, TRUST_DECISION_UT (all three)
//	RTL_RandomForest -> <TRUST_DECISION_MODEL_DIR>/<config>/rf_model.json, or TRUST_DECISION_RF_MODEL (explicit path)
//	RTL_Optuna       -> <TRUST_DECISION_MODEL_DIR>/<config>/rtl_optuna.json, or TRUST_DECISION_OPTUNA_MODEL (explicit path)
//	TRUST_DECISION_MODEL_DIR defaults to /opt/rtl-models; <config> = <TD>-<TF>-<TQM>[-<window>] of this TAF (rtlConfigName).
//
// It panics if a mandatory parameter is missing or invalid, or if the trained model cannot be read.
func newRTLTrustDecider(kind string, params map[string]string) trustDeciderFunc {
	// Decision logic.
	usePP := false
	if logic, exists := params["TRUST_DECISION_LOGIC"]; exists {
		switch logic {
		case "Components":
		case "ProjectedProbability":
			usePP = true
		default:
			panic("Could not parse TRUST_DECISION_LOGIC")
		}
	}

	// RTL source: returns the RTL for a given ATL.
	var rtlFor func(rtl.ATL) rtl.RTL
	switch kind {
	case "RTL_RiskBased":
		fixed := rtl.RTL{
			Bt: thresholdParam(params, "TRUST_DECISION_BT"),
			Dt: thresholdParam(params, "TRUST_DECISION_DT"),
			Ut: thresholdParam(params, "TRUST_DECISION_UT"),
		}
		rtlFor = func(rtl.ATL) rtl.RTL { return fixed }
	case "RTL_RandomForest":
		path, err := modelPath(params, "TRUST_DECISION_RF_MODEL", rfModelFile)
		if err != nil {
			panic("Could not read RTL model: " + err.Error())
		}
		model, err := loadRFModel(path)
		if err != nil {
			panic("Could not read RTL model: " + err.Error())
		}
		rtlFor = model.PredictRTL
	case "RTL_Optuna":
		path, err := modelPath(params, "TRUST_DECISION_OPTUNA_MODEL", optunaModelFile)
		if err != nil {
			panic("Could not read RTL model: " + err.Error())
		}
		fixed, err := loadOptunaRTL(path)
		if err != nil {
			panic("Could not read RTL model: " + err.Error())
		}
		rtlFor = func(rtl.ATL) rtl.RTL { return fixed }
	default:
		panic("Unknown RTL trust decision: " + kind)
	}

	return func(proposition string, atlOpinion subjectivelogic.QueryableOpinion, _ subjectivelogic.QueryableOpinion) core.TrustDecision {
		if atlOpinion.Uncertainty() == 1 {
			return core.UNDECIDABLE
		}
		atl := rtl.ATL{B: atlOpinion.Belief(), D: atlOpinion.Disbelief(), U: atlOpinion.Uncertainty()}
		required := rtlFor(atl)

		trusted := false
		if usePP {
			// Gray zone (PUT <= P <= PTT) is rejected.
			trusted = rtl.DecidePP(atl, required, atlOpinion.BaseRate()).Trusted
		} else {
			trusted = rtl.Decide(atl, required)
		}
		if trusted {
			return core.TRUSTWORTHY
		}
		return core.NOT_TRUSTWORTHY
	}
}
