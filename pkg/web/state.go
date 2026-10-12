package web

import (
	"cmp"
	"encoding/json"
	"log/slog"
	"maps"
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/vs-uulm/go-subjectivelogic/pkg/subjectivelogic"
	"github.com/vs-uulm/go-taf/internal/util"
	"github.com/vs-uulm/go-taf/pkg/core"
	"github.com/vs-uulm/go-taf/pkg/listener"
	"github.com/vs-uulm/go-taf/pkg/trustmodel/trustmodelstructure"
)

/*
The State struct is a copy of the TAF internal state, built from an event stream sent from the TAF.
This copy can then be used by the WEB UI to give read access to the recreated TAF state.
This includes:
  - stream of internal events sent from the TAF
  - state of TMIs

The state is written by State.Handle only and read by the HTTP handlers, which run in other go-routines, so all access
is guarded by mutex. Events are stored as JSON snapshots marshaled once upon arrival: they are immutable afterwards, so
HTTP handlers only need to hold the lock while collecting references and can serialize their response without it.
*/
type State struct {
	mutex    sync.RWMutex
	eventLog []json.RawMessage
	logger   *slog.Logger
	tmis     map[string]*tmiMetaState
	sessions map[string]*sessionState
}

func NewState(logger *slog.Logger) *State {
	return &State{
		logger:   logger,
		eventLog: make([]json.RawMessage, 0),
		tmis:     make(map[string]*tmiMetaState),
		sessions: make(map[string]*sessionState),
	}
}

/*
PAGE_SIZE defines the maximum amount of items shown with pagination.
*/
const PAGE_SIZE = 100

type tmiState struct {
	Version     int
	Fingerprint uint32
	Structure   trustmodelstructure.TrustGraphStructure
	Values      map[string][]trustmodelstructure.TrustRelationship
	RTLs        map[string]subjectivelogic.QueryableOpinion
}

type tmiMetaState struct {
	ID            string
	FullTMI       string
	IsActive      bool
	LatestVersion int
	Update        map[int][]json.RawMessage // marshaled core.Update
	States        map[int]json.RawMessage   // marshaled tmiState
	Template      string
	ATLs          map[int]json.RawMessage // marshaled core.AtlResultSet
	// Decisions holds the trust decisions contained in the ATL result set of each version (a bit per decision, see
	// decisionBit); DecisionCounts counts the versions whose result set contains each decision.
	Decisions      map[int]uint8
	DecisionCounts map[core.TrustDecision]int
	// UpdateTimes holds the time of each update in Update (same index), SpawnTime the time of version 0,
	// LatestUpdateTime the latest of all these times, and DeleteTime the time of the deletion (0 while active), all in
	// nanoseconds as given by package clock (simulated time in simulation builds, Unix time otherwise).
	UpdateTimes      map[int][]int64
	SpawnTime        int64
	LatestUpdateTime int64
	DeleteTime       int64
}

/*
versionTime returns the time of a version: the time of its latest update, or the time of the spawn for version 0.
*/
func (tmi *tmiMetaState) versionTime(version int) (int64, bool) {
	if times := tmi.UpdateTimes[version]; len(times) > 0 {
		return slices.Max(times), true
	}
	if version == 0 {
		return tmi.SpawnTime, true
	}
	return 0, false
}

/*
versionAt returns the latest version of the TMI at the given time, i.e., the highest version whose time is not later,
or false if the TMI has not been spawned by then.
*/
func (tmi *tmiMetaState) versionAt(at int64) (int, int64, bool) {
	if tmi.SpawnTime > at {
		return 0, 0, false
	}
	version, versionTime := 0, tmi.SpawnTime
	for v := range tmi.States {
		if t, ok := tmi.versionTime(v); ok && t <= at && v > version {
			version, versionTime = v, t
		}
	}
	return version, versionTime, true
}

/*
decisionsAt returns the trust decisions of the given version (bits as in Decisions) and the version they belong to: those
of the version itself, or of the closest earlier version with ATLs if it has none yet; -1 if no version up to it has any.
*/
func (tmi *tmiMetaState) decisionsAt(version int) (uint8, int) {
	for v := version; v >= 0; v-- {
		if bits := tmi.Decisions[v]; bits != 0 {
			return bits, v
		}
	}
	return 0, -1
}

// decisionCountsUpTo returns the number of versions up to the given one per trust decision, keyed by decision name.
func (tmi *tmiMetaState) decisionCountsUpTo(version int) map[string]int {
	counts := make(map[string]int, len(trustDecisions))
	for _, d := range trustDecisions {
		counts[d.name] = 0
	}
	for v, bits := range tmi.Decisions {
		if v > version {
			continue
		}
		for _, d := range trustDecisions {
			if bits&decisionBit(d.decision) != 0 {
				counts[d.name]++
			}
		}
	}
	return counts
}

// trustDecisions lists all trust decisions with their names in the TMI overview.
var trustDecisions = []struct {
	decision core.TrustDecision
	name     string
}{
	{core.TRUSTWORTHY, "TRUSTWORTHY"},
	{core.NOT_TRUSTWORTHY, "NOT_TRUSTWORTHY"},
	{core.UNDECIDABLE, "UNDECIDABLE"},
}

func decisionBit(decision core.TrustDecision) uint8 {
	return 1 << decision
}

/*
countDecisions sets the trust decisions of a version from its ATL result set and updates the per-decision counts. A
version counts once per decision it contains, also if several propositions share it; a later result set for the same
version replaces the earlier one.
*/
func (tmi *tmiMetaState) countDecisions(version int, decisions map[string]core.TrustDecision) {
	var bits uint8
	for _, decision := range decisions {
		bits |= decisionBit(decision)
	}
	previous := tmi.Decisions[version]
	for _, d := range trustDecisions {
		if previous&decisionBit(d.decision) != 0 {
			tmi.DecisionCounts[d.decision]--
		}
		if bits&decisionBit(d.decision) != 0 {
			tmi.DecisionCounts[d.decision]++
		}
	}
	tmi.Decisions[version] = bits
}

// decisionByName returns the trust decision with the given name, as used in the TMI overview.
func decisionByName(name string) (core.TrustDecision, bool) {
	for _, d := range trustDecisions {
		if d.name == name {
			return d.decision, true
		}
	}
	return 0, false
}

// decisionCountsByName returns the number of versions per trust decision, keyed by decision name.
func (tmi *tmiMetaState) decisionCountsByName() map[string]int {
	counts := make(map[string]int, len(trustDecisions))
	for _, d := range trustDecisions {
		counts[d.name] = tmi.DecisionCounts[d.decision]
	}
	return counts
}

type sessionState struct {
	Client   string
	IsActive bool
	TMIs     []string
	Template string
}

/*
Handle processes all events from the TAF and the web socket clients. Events from the TAF are applied to the state and
forwarded to all connected web socket clients. Clients that cannot keep up are disconnected.
*/
func (s *State) Handle(incomingEvents *eventQueue, websocketEvents chan WebSocketEvent) {
	clients := make(map[*webSocketClient]struct{})
	for {
		select {
		case <-incomingEvents.signal:
			for _, evt := range incomingEvents.drain() {
				msg, err := s.apply(evt)
				if err != nil {
					s.logger.Error("could not process event", "error", err)
					continue
				}

				for client := range clients {
					select {
					case client.send <- msg:
					default:
						s.logger.Warn("disconnecting web socket client that cannot keep up")
						delete(clients, client)
						close(client.send)
						client.conn.Close()
					}
				}
			}

		case evt := <-websocketEvents:
			switch evt.Type {
			case CONNECTED:
				clients[evt.Client] = struct{}{}
			case DISCONNECTED:
				if _, exists := clients[evt.Client]; exists {
					delete(clients, evt.Client)
					close(evt.Client.send)
				}
			}
		}
	}
}

/*
apply adds an event to the state and returns its JSON representation.
Events (and the TMI structures, values, and updates they contain) are marshaled here, i.e., in the web server
go-routine and not upon emission. This relies on TMIs replacing their structure and values upon changes instead of
modifying the previously returned instances.
*/
func (s *State) apply(evt listener.ListenerEvent) (json.RawMessage, error) {
	msg, err := json.Marshal(evt)
	if err != nil {
		return nil, err
	}

	switch event := evt.(type) {
	case listener.ATLRemovedEvent:
		s.logger.Debug("ATLRemovedEvent")
	case listener.ATLUpdatedEvent:
		s.logger.Debug("ATLUpdatedEvent", "atls", event.NewATLs)
		err = s.handleATLUpdatedEvent(event)
	case listener.TrustModelInstanceSpawnedEvent:
		s.logger.Debug("TrustModelInstanceSpawnedEvent")
		err = s.handleTMISpawned(event)
	case listener.TrustModelInstanceUpdatedEvent:
		s.logger.Debug("TrustModelInstanceUpdatedEvent")
		err = s.handleTMIUpdated(event)
	case listener.TrustModelInstanceDeletedEvent:
		s.logger.Debug("TrustModelInstanceDeletedEvent")
		s.handleTMIDeleted(event)
	case listener.SessionCreatedEvent:
		s.logger.Debug("SessionCreatedEvent")
		s.handleSessionCreatedEvent(event)
	case listener.SessionTorndownEvent:
		s.logger.Debug("SessionTorndownEvent")
		s.handleSessionTorndownEvent(event)
	default:
		util.UNUSED(event)
	}
	if err != nil {
		return nil, err
	}

	s.mutex.Lock()
	s.eventLog = append(s.eventLog, msg)
	s.mutex.Unlock()
	return msg, nil
}

func (s *State) handleSessionCreatedEvent(event listener.SessionCreatedEvent) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.sessions[event.SessionID] = &sessionState{
		Client:   event.ClientID,
		IsActive: true,
		TMIs:     make([]string, 0),
		Template: event.TrustModelTemplate,
	}
}

func (s *State) handleSessionTorndownEvent(event listener.SessionTorndownEvent) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if _, exists := s.sessions[event.SessionID]; exists {
		s.sessions[event.SessionID].IsActive = false
	}
}

func (s *State) handleATLUpdatedEvent(event listener.ATLUpdatedEvent) error {
	atls, err := json.Marshal(event.NewATLs)
	if err != nil {
		return err
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()
	tmi, exists := s.tmis[event.FullTMI]
	if !exists {
		return nil
	}
	tmi.ATLs[event.NewATLs.Version()] = atls
	tmi.countDecisions(event.NewATLs.Version(), event.NewATLs.TrustDecisions())
	return nil
}

func (s *State) handleTMISpawned(event listener.TrustModelInstanceSpawnedEvent) error {
	state, err := json.Marshal(tmiState{
		Version:     event.Version,
		Fingerprint: event.Fingerprint,
		Structure:   event.Structure,
		Values:      event.Values,
		RTLs:        event.RTLs,
	})
	if err != nil {
		return err
	}

	fullTMI := event.FullTMI
	_, sessionID, _, _ := core.SplitFullTMIIdentifier(fullTMI)

	s.mutex.Lock()
	defer s.mutex.Unlock()
	if _, exists := s.sessions[sessionID]; exists {
		s.sessions[sessionID].TMIs = append(s.sessions[sessionID].TMIs, fullTMI)
	}

	s.tmis[fullTMI] = &tmiMetaState{
		IsActive:       true,
		LatestVersion:  0,
		Update:         make(map[int][]json.RawMessage),
		UpdateTimes:    make(map[int][]int64),
		SpawnTime:      event.Time,
		States:         map[int]json.RawMessage{event.Version: state},
		Template:       event.Template.Identifier(),
		ID:             event.ID,
		FullTMI:        event.FullTMI,
		ATLs:           make(map[int]json.RawMessage),
		Decisions:      make(map[int]uint8),
		DecisionCounts: make(map[core.TrustDecision]int),
	}
	s.tmis[fullTMI].LatestUpdateTime = event.Time
	return nil
}

func (s *State) handleTMIUpdated(event listener.TrustModelInstanceUpdatedEvent) error {
	update, err := json.Marshal(event.Update)
	if err != nil {
		return err
	}

	s.mutex.RLock()
	tmi, exists := s.tmis[event.FullTMI]
	var versionExists bool
	if exists {
		_, versionExists = tmi.States[event.Version]
	}
	s.mutex.RUnlock()
	if !exists {
		return nil
	}

	// If there exists already an entry for that version, this means we have received another update that yields the same
	// version number. This means that the second update has failed to increase the version number and can be ignored.
	if versionExists {
		s.mutex.Lock()
		tmi.Update[event.Version+1] = []json.RawMessage{update}
		tmi.UpdateTimes[event.Version+1] = []int64{event.Time}
		tmi.LatestUpdateTime = max(tmi.LatestUpdateTime, event.Time)
		s.mutex.Unlock()
		return nil
	}

	state, err := json.Marshal(tmiState{
		Version:     event.Version,
		Fingerprint: event.Fingerprint,
		Structure:   event.Structure,
		Values:      event.Values,
		RTLs:        event.RTLs,
	})
	if err != nil {
		return err
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()
	tmi.States[event.Version] = state
	tmi.Update[event.Version] = append(tmi.Update[event.Version], update)
	tmi.UpdateTimes[event.Version] = append(tmi.UpdateTimes[event.Version], event.Time)
	tmi.LatestUpdateTime = max(tmi.LatestUpdateTime, event.Time)
	tmi.LatestVersion = event.Version
	return nil
}

func (s *State) handleTMIDeleted(event listener.TrustModelInstanceDeletedEvent) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if tmi, exists := s.tmis[event.FullTMI]; exists {
		tmi.IsActive = false
		tmi.DeleteTime = event.Time
	}
}

func (s *State) getSessions(ctx *gin.Context) {
	s.mutex.RLock()
	sessions := make(map[string]sessionState, len(s.sessions))
	for id, session := range s.sessions {
		sessions[id] = *session
	}
	s.mutex.RUnlock()
	ctx.JSON(http.StatusOK, sessions)
}

/*
events returns the current event log. As the event log is append-only and its entries are immutable, the returned
slice can be read without holding the lock.
*/
func (s *State) events() []json.RawMessage {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.eventLog[:len(s.eventLog):len(s.eventLog)]
}

func (s *State) getFullEventLog(ctx *gin.Context) {
	eventLog := s.events()
	fullLog := make([]map[int]interface{}, len(eventLog))

	logLength := len(eventLog)
	for i, entry := range eventLog {
		fullLog[logLength-i-1] = map[int]interface{}{
			i: entry,
		}
	}
	ctx.JSON(http.StatusOK, fullLog)
}

func (s *State) getEventLogPage(ctx *gin.Context) {
	eventLog := s.events()

	var cursor int
	rawCursor, exists := ctx.GetQuery("cursor")
	if !exists {
		ctx.Redirect(http.StatusFound, ctx.Request.URL.Path+"?cursor=0")
		return
	} else {
		c, err := strconv.Atoi(rawCursor)
		if err != nil {
			ctx.JSON(http.StatusNotFound, gin.H{"code": "INVALID_CURSOR"})
			return
		} else {
			cursor = c
		}
	}

	lower := max(0, cursor)
	upper := min(len(eventLog), cursor+PAGE_SIZE)

	if upper-lower == 0 {
		ctx.JSON(http.StatusOK, make([]interface{}, 0))
		return
	} else if lower > upper {
		ctx.JSON(http.StatusOK, gin.H{"code": "INVALID_CURSOR"})
		return
	}
	log := make([]map[int]interface{}, upper-lower)

	for i := lower; i <= upper-1; i++ {
		log[upper-i-1] = map[int]interface{}{
			i: eventLog[i],
		}
	}

	res := gin.H{"page": log}
	prev := max(lower-PAGE_SIZE, 0)
	if cursor != prev {
		res["previous"] = prev
	}
	if next := lower + PAGE_SIZE; next < len(eventLog) {
		res["next"] = next
	}
	ctx.JSON(http.StatusOK, res)
}

func (s *State) getLatestEventLogPage(ctx *gin.Context) {
	eventLog := s.events()

	upper := len(eventLog)
	lower := max(0, upper-PAGE_SIZE)

	log := make([]map[int]interface{}, upper-lower)

	if upper-lower == 0 {
		ctx.JSON(http.StatusOK, log)
		return
	}

	for i := lower; i <= upper-1; i++ {
		log[upper-i-1] = map[int]interface{}{
			i: eventLog[i],
		}
	}
	ctx.JSON(http.StatusOK, log)
}

/*
tmi returns a copy of the meta state of a TMI with copies of its version maps, so that it can be serialized without
holding the lock. The contained snapshots are immutable.
*/
func (s *State) tmi(ctx *gin.Context) (tmiMetaState, bool) {
	fullTMI := core.MergeFullTMIIdentifier(ctx.Param("client"), ctx.Param("session"), ctx.Param("tmt"), ctx.Param("tmiID"))
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	tmi, exists := s.tmis[fullTMI]
	if !exists {
		return tmiMetaState{}, false
	}
	result := *tmi
	result.States = maps.Clone(tmi.States)
	result.Update = maps.Clone(tmi.Update)
	result.UpdateTimes = maps.Clone(tmi.UpdateTimes)
	result.ATLs = maps.Clone(tmi.ATLs)
	result.Decisions = maps.Clone(tmi.Decisions)
	result.DecisionCounts = maps.Clone(tmi.DecisionCounts)
	return result, true
}

func (s *State) getTMI(ctx *gin.Context) {
	_, exists := s.tmi(ctx)
	if !exists {
		ctx.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND"})
	} else {
		ctx.Redirect(http.StatusFound, ctx.Request.URL.Path+"/latest")
	}
}

func (s *State) getTMIFull(ctx *gin.Context) {
	tmi, exists := s.tmi(ctx)
	if !exists {
		ctx.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND"})
		return
	} else {
		res := gin.H{
			"id":          tmi.ID,
			"fullTMI":     tmi.FullTMI,
			"active":      tmi.IsActive,
			"template":    tmi.Template,
			"states":      tmi.States,
			"updates":     tmi.Update,
			"updateTimes": tmi.UpdateTimes,
			"spawnTime":   tmi.SpawnTime,
			"atls":        tmi.ATLs,
		}

		ctx.JSON(http.StatusOK, res)
	}

}

func (s *State) getTMIUpdates(ctx *gin.Context) {
	tmi, exists := s.tmi(ctx)
	if !exists {
		ctx.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND"})
		return
	} else {
		ctx.JSON(http.StatusOK, tmi.Update)
	}

}

func (s *State) getVersionTMI(ctx *gin.Context) {
	tmi, exists := s.tmi(ctx)
	if !exists {
		ctx.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND"})
	} else {
		version, err := strconv.Atoi(ctx.Param("version"))
		if err != nil {
			ctx.JSON(http.StatusNotFound, gin.H{"code": "VERSION_NOT_FOUND"})
			return
		}
		if _, exists := tmi.States[version]; !exists {
			ctx.JSON(http.StatusNotFound, gin.H{"code": "VERSION_NOT_FOUND"})
			return
		}
		ctx.JSON(http.StatusOK, versionResponse(tmi, version))
	}
}

func (s *State) getTMILatest(ctx *gin.Context) {
	tmi, exists := s.tmi(ctx)
	if !exists {
		ctx.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND"})
	} else {
		ctx.JSON(http.StatusOK, versionResponse(tmi, tmi.LatestVersion))
	}
}

/*
VERSIONS_PAGE_SIZE defines the default and MAX_VERSIONS_PAGE_SIZE the maximum amount of TMI versions returned at once.
*/
const VERSIONS_PAGE_SIZE = 100
const MAX_VERSIONS_PAGE_SIZE = 1000

/*
getTMIVersions returns the versions of a TMI (state, updates, and ATLs per version) in descending order, page by page.
The optional query parameter before only returns versions older than the given version, limit sets the page size.
The optional query parameter search only returns versions whose number or updates contain the given term (ignoring case).
The optional query parameter decision only returns versions whose ATL result set contains the given trust decision
(TRUSTWORTHY, NOT_TRUSTWORTHY, or UNDECIDABLE); search and decision can be combined.
*/
func (s *State) getTMIVersions(ctx *gin.Context) {
	tmi, exists := s.tmi(ctx)
	if !exists {
		ctx.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND"})
		return
	}

	limit := VERSIONS_PAGE_SIZE
	if rawLimit, exists := ctx.GetQuery("limit"); exists {
		l, err := strconv.Atoi(rawLimit)
		if err != nil || l <= 0 {
			ctx.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_LIMIT"})
			return
		}
		limit = min(l, MAX_VERSIONS_PAGE_SIZE)
	}
	before := math.MaxInt
	if rawBefore, exists := ctx.GetQuery("before"); exists {
		b, err := strconv.Atoi(rawBefore)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_VERSION"})
			return
		}
		before = b
	}

	var decisionFilter uint8
	if rawDecision, exists := ctx.GetQuery("decision"); exists && rawDecision != "" {
		decision, valid := decisionByName(rawDecision)
		if !valid {
			ctx.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_DECISION"})
			return
		}
		decisionFilter = decisionBit(decision)
	}

	versions := slices.Sorted(maps.Keys(tmi.States))
	slices.Reverse(versions)
	total := len(versions)
	if decisionFilter != 0 {
		versions = slices.DeleteFunc(versions, func(version int) bool { return tmi.Decisions[version]&decisionFilter == 0 })
	}
	if search, exists := ctx.GetQuery("search"); exists && search != "" {
		versions = slices.DeleteFunc(versions, func(version int) bool { return !versionMatches(tmi, version, search) })
	}
	start, found := slices.BinarySearchFunc(versions, before, func(version int, target int) int { return target - version })
	if found {
		start++ // before is exclusive
	}
	page := versions[start:min(start+limit, len(versions))]

	items := make([]gin.H, len(page))
	for i, version := range page {
		items[i] = gin.H{
			"version":     version,
			"state":       tmi.States[version],
			"updates":     tmi.Update[version],
			"updateTimes": tmi.UpdateTimes[version],
			"atls":        tmi.ATLs[version],
		}
	}
	ctx.JSON(http.StatusOK, gin.H{
		"id":            tmi.ID,
		"fullTMI":       tmi.FullTMI,
		"active":        tmi.IsActive,
		"template":      tmi.Template,
		"latestVersion": tmi.LatestVersion,
		"spawnTime":     tmi.SpawnTime,
		"total":         total,
		"matches":       len(versions),
		"versions":      items,
	})
}

/*
versionMatches checks whether the number or the updates of a TMI version contain the search term, ignoring case. As
updates are stored as compact JSON, but shown with indentation, the term is also matched with its whitespace removed.
*/
func versionMatches(tmi tmiMetaState, version int, search string) bool {
	term := strings.ToLower(search)
	if strings.Contains(strconv.Itoa(version), term) {
		return true
	}
	compactTerm := strings.Join(strings.Fields(term), "")
	for _, update := range tmi.Update[version] {
		lower := strings.ToLower(string(update))
		if strings.Contains(lower, term) || (compactTerm != "" && strings.Contains(lower, compactTerm)) {
			return true
		}
	}
	return false
}

func versionResponse(tmi tmiMetaState, version int) gin.H {
	state, exists := tmi.States[version]
	if !exists {
		// same as serializing a missing entry of a map[int]tmiState
		state, _ = json.Marshal(tmiState{})
	}
	res := gin.H{
		"id":          tmi.ID,
		"fullTMI":     tmi.FullTMI,
		"active":      tmi.IsActive,
		"template":    tmi.Template,
		"state":       state,
		"updates":     tmi.Update[version],
		"updateTimes": tmi.UpdateTimes[version],
		"spawnTime":   tmi.SpawnTime,
	}

	atls, atlExist := tmi.ATLs[version]
	if !atlExist {
		res["atls"] = nil
	} else {
		res["atls"] = atls
	}
	return res
}

/*
getTMIDecisions returns the trust decisions of all versions of a TMI as one number per version (index = version), with
bit 1<<d set for every trust decision d (core.TrustDecision) in the version's ATL result set and 0 for versions without
one, e.g., for coloring a timeline of the versions.
*/
func (s *State) getTMIDecisions(ctx *gin.Context) {
	tmi, exists := s.tmi(ctx)
	if !exists {
		ctx.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND"})
		return
	}
	last := tmi.LatestVersion
	for version := range tmi.Decisions {
		last = max(last, version)
	}
	// ints instead of uint8s, as a []uint8 would be marshaled as base64
	decisions := make([]int, last+1)
	for version, bits := range tmi.Decisions {
		if version >= 0 {
			decisions[version] = int(bits)
		}
	}
	ctx.JSON(http.StatusOK, gin.H{
		"latestVersion": tmi.LatestVersion,
		"decisions":     decisions,
	})
}

/*
getAllTMIs returns a summary of every TMI. With the optional query parameter at (a time in nanoseconds, see package
clock), it returns the TMIs as they were at that time instead: only those spawned by then, each with its latest version
at that time, the time of that version, the trust decisions of the versions up to it, and whether it was still active.
*/
func (s *State) getAllTMIs(ctx *gin.Context) {
	type tmiSummary struct {
		Id            string `json:"id"`
		FullTMI       string `json:"fullTMI"`
		Active        bool   `json:"active"`
		Template      string `json:"template"`
		LatestVersion int    `json:"latestVersion"`
		// number of versions whose ATL result set contains each trust decision
		DecisionCounts map[string]int `json:"decisionCounts"`
		// time of the latest update (or of the spawn) and of the spawn, see tmiMetaState
		LatestUpdateTime int64 `json:"latestUpdateTime"`
		SpawnTime        int64 `json:"spawnTime"`
		// trust decisions of the latest version (see tmiMetaState.decisionsAt) and the version they belong to
		LatestDecisions        int `json:"latestDecisions"`
		LatestDecisionsVersion int `json:"latestDecisionsVersion"`
	}

	at, timed := int64(0), false
	if rawAt, exists := ctx.GetQuery("at"); exists && rawAt != "" {
		a, err := strconv.ParseInt(rawAt, 10, 64)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_TIME"})
			return
		}
		at, timed = a, true
	}

	s.mutex.RLock()
	tmis := make(map[string]tmiSummary, len(s.tmis))
	for fullTMI, tmi := range s.tmis {
		if !timed {
			decisions, decisionsVersion := tmi.decisionsAt(tmi.LatestVersion)
			tmis[fullTMI] = tmiSummary{tmi.ID, fullTMI, tmi.IsActive, tmi.Template, tmi.LatestVersion, tmi.decisionCountsByName(), tmi.LatestUpdateTime, tmi.SpawnTime, int(decisions), decisionsVersion}
			continue
		}
		version, versionTime, spawned := tmi.versionAt(at)
		if !spawned {
			continue
		}
		active := tmi.IsActive || tmi.DeleteTime > at
		decisions, decisionsVersion := tmi.decisionsAt(version)
		tmis[fullTMI] = tmiSummary{tmi.ID, fullTMI, active, tmi.Template, version, tmi.decisionCountsUpTo(version), versionTime, tmi.SpawnTime, int(decisions), decisionsVersion}
	}
	s.mutex.RUnlock()
	ctx.JSON(http.StatusOK, tmis)
}

/*
getDecisionTimeline returns how the trust decisions of all TMIs together changed over time, as segments in ascending
order, each with its start time (in nanoseconds, see package clock) and the trust decisions contained in the current
versions of all TMIs active at that time (bit 1<<d per core.TrustDecision d, as in getTMIDecisions). A version without
ATLs keeps the decisions of the previous version of its TMI, and deleted TMIs no longer count.
*/
func (s *State) getDecisionTimeline(ctx *gin.Context) {
	type change struct {
		time int64
		tmi  int
		bits uint8
	}

	s.mutex.RLock()
	changes := make([]change, 0)
	tmiCount := 0
	for _, tmi := range s.tmis {
		var bits uint8
		for _, version := range slices.Sorted(maps.Keys(tmi.States)) {
			time, ok := tmi.versionTime(version)
			if !ok {
				continue
			}
			if tmi.Decisions[version] != 0 {
				bits = tmi.Decisions[version]
			}
			changes = append(changes, change{time, tmiCount, bits})
		}
		if !tmi.IsActive {
			changes = append(changes, change{tmi.DeleteTime, tmiCount, 0})
		}
		tmiCount++
	}
	s.mutex.RUnlock()

	// stable, so that the changes of a TMI at the same time are applied in the order of its versions
	slices.SortStableFunc(changes, func(a, b change) int { return cmp.Compare(a.time, b.time) })

	type segment struct {
		Time      int64 `json:"time"`
		Decisions int   `json:"decisions"`
	}
	segments := make([]segment, 0)
	current := make([]uint8, tmiCount)
	// number of TMIs whose current version contains each decision bit
	var counts [8]int
	for i := 0; i < len(changes); {
		time := changes[i].time
		for ; i < len(changes) && changes[i].time == time; i++ {
			c := changes[i]
			for bit := range counts {
				if current[c.tmi]&(1<<bit) != 0 {
					counts[bit]--
				}
				if c.bits&(1<<bit) != 0 {
					counts[bit]++
				}
			}
			current[c.tmi] = c.bits
		}
		decisions := 0
		for bit, count := range counts {
			if count > 0 {
				decisions |= 1 << bit
			}
		}
		if len(segments) == 0 || segments[len(segments)-1].Decisions != decisions {
			segments = append(segments, segment{time, decisions})
		}
	}
	ctx.JSON(http.StatusOK, gin.H{"segments": segments})
}
