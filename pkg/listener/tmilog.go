package listener

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/vs-uulm/go-subjectivelogic/pkg/subjectivelogic"
	"github.com/vs-uulm/go-taf/pkg/trustmodel/trustmodelstructure"
)

/*
TMILogEnvVariable names the environment variable that enables the TMI log: the path of the file it writes to.
*/
const TMILogEnvVariable = "TAF_TMI_LOG"

/*
TMILog is a TrustModelInstanceListener that appends one JSON line per spawned or updated TMI to a file: the time of the
event (as given by package clock), the TMI's ID, full ID and version, and its trust relationships per scope with their
opinions. A result of the TMI (e.g., in a TAS_TA_RESPONSE) can be matched to the relationships it was derived from by
TMI ID and version.
*/
type TMILog struct {
	mutex sync.Mutex
	file  *os.File
}

type tmiLogOpinion struct {
	Belief      float64 `json:"belief"`
	Disbelief   float64 `json:"disbelief"`
	Uncertainty float64 `json:"uncertainty"`
	BaseRate    float64 `json:"baseRate"`
}

type tmiLogRelationship struct {
	Source      string         `json:"source"`
	Destination string         `json:"destination"`
	Opinion     *tmiLogOpinion `json:"opinion"`
}

type tmiLogEntry struct {
	Event   string                          `json:"event"`
	Time    int64                           `json:"time"`
	ID      string                          `json:"id"`
	FullTMI string                          `json:"fullTmi"`
	Version int                             `json:"version"`
	Values  map[string][]tmiLogRelationship `json:"values"`
}

/*
NewTMILogFromEnv creates a TMILog writing to the file named by the environment variable TAF_TMI_LOG, truncating it. It
returns nil if the variable is not set, so that no listener is registered and the TAF builds no events for it.
*/
func NewTMILogFromEnv() (*TMILog, error) {
	path := os.Getenv(TMILogEnvVariable)
	if path == "" {
		return nil, nil
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("opening the TMI log %s: %w", path, err)
	}
	return &TMILog{file: file}, nil
}

func (l *TMILog) OnTrustModelInstanceSpawned(event TrustModelInstanceSpawnedEvent) {
	l.write(tmiLogEntry{Event: "spawned", Time: event.Time, ID: event.ID, FullTMI: event.FullTMI,
		Version: event.Version, Values: tmiLogValues(event.Values)})
}

func (l *TMILog) OnTrustModelInstanceUpdated(event TrustModelInstanceUpdatedEvent) {
	l.write(tmiLogEntry{Event: "updated", Time: event.Time, ID: event.ID, FullTMI: event.FullTMI,
		Version: event.Version, Values: tmiLogValues(event.Values)})
}

func (l *TMILog) OnTrustModelInstanceDeleted(event TrustModelInstanceDeletedEvent) {
	// the relationships of a deleted TMI do not change anymore
}

/*
write appends the entry as one line. The workers call the listener concurrently; every line is written with a single
call, so a run that ends abruptly loses no line already written.
*/
func (l *TMILog) write(entry tmiLogEntry) {
	line, err := json.Marshal(entry)
	if err != nil {
		return
	}
	line = append(line, '\n')
	l.mutex.Lock()
	defer l.mutex.Unlock()
	_, _ = l.file.Write(line)
}

func tmiLogValues(values map[string][]trustmodelstructure.TrustRelationship) map[string][]tmiLogRelationship {
	result := make(map[string][]tmiLogRelationship, len(values))
	for scope, relationships := range values {
		entries := make([]tmiLogRelationship, 0, len(relationships))
		for _, relationship := range relationships {
			entries = append(entries, tmiLogRelationship{
				Source:      relationship.Source(),
				Destination: relationship.Destination(),
				Opinion:     tmiLogOpinionOf(relationship.Opinion()),
			})
		}
		result[scope] = entries
	}
	return result
}

func tmiLogOpinionOf(opinion subjectivelogic.QueryableOpinion) *tmiLogOpinion {
	if opinion == nil {
		return nil
	}
	return &tmiLogOpinion{
		Belief:      opinion.Belief(),
		Disbelief:   opinion.Disbelief(),
		Uncertainty: opinion.Uncertainty(),
		BaseRate:    opinion.BaseRate(),
	}
}
