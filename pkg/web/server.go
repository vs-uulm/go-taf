package web

import (
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	logging "github.com/vs-uulm/go-taf/internal/logger"
	"github.com/vs-uulm/go-taf/internal/version"
	"github.com/vs-uulm/go-taf/pkg/core"
	"github.com/vs-uulm/go-taf/pkg/manager"
)

//go:embed frontend/dist
var webFrontend embed.FS

//https://www.jetbrains.com/guide/go/tutorials/rest_api_series/gin/

type Webserver struct {
	tafContext       core.TafContext
	logger           *slog.Logger
	router           *gin.Engine
	events           *eventQueue
	websocketChannel chan WebSocketEvent
	state            *State
	tmts             map[string]interface{}
	trustSources     map[string]map[string]bool
}

func New(tafContext core.TafContext) (*Webserver, error) {
	logger := logging.CreateChildLogger(tafContext.Logger, "WEB-UI")
	return &Webserver{
		tafContext:       tafContext,
		logger:           logger,
		events:           newEventQueue(),
		websocketChannel: make(chan WebSocketEvent),
		state:            NewState(logger),
		tmts:             make(map[string]interface{}),
		trustSources:     make(map[string]map[string]bool),
	}, nil
}

type WebSocketConnectedEvent struct {
	Timestamp  time.Time
	Connection *websocket.Conn
}

func (s *Webserver) Run() {
	go s.state.Handle(s.events, s.websocketChannel)

	staticFS := fs.FS(webFrontend)
	frontendDir, err := fs.Sub(staticFS, "frontend/dist")
	if err != nil {
		panic(err)
	}

	gin.SetMode(gin.ReleaseMode) //Disable Gin-specific logging output
	s.router = gin.New()         //Create a non-default router without request logging
	s.router.Use(func(c *gin.Context) {
		s.logger.Debug("gin request", "uri", c.Request.RequestURI)
		c.Next()
	})

	s.router.Use(gin.Recovery())
	s.router.GET("/ui/*filepath", frontendHandler(frontendDir))
	s.router.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/ui")
	})
	s.router.GET("/ws", s.handleWebSocket)

	//	s.router.LoadHTMLGlob("res/templates/*")
	s.router.GET("/api/events", s.state.getEventLogPage)
	s.router.GET("/api/events/latest", s.state.getLatestEventLogPage)
	s.router.GET("/api/events/all", s.state.getFullEventLog)
	s.router.GET("/api/info", s.getInfo)
	s.router.GET("/api/sessions", s.state.getSessions)
	s.router.GET("/api/tmis/", s.state.getAllTMIs)
	s.router.GET("/api/tmis/:client/:session/:tmt/:tmiID", s.state.getTMI)
	s.router.GET("/api/tmis/:client/:session/:tmt/:tmiID/latest", s.state.getTMILatest)
	s.router.GET("/api/tmis/:client/:session/:tmt/:tmiID/updates", s.state.getTMIUpdates)
	s.router.GET("/api/tmis/:client/:session/:tmt/:tmiID/all", s.state.getTMIFull)
	s.router.GET("/api/tmis/:client/:session/:tmt/:tmiID/:version", s.state.getVersionTMI)
	s.router.GET("/api/trustmodels/:tmt-identifier", s.getTrustModel)
	s.router.GET("/api/trustsources", s.getTrustSources)
	s.router.GET("/api/trustmodels", s.getTrustModels)
	s.router.Run(fmt.Sprintf(":%d", s.tafContext.Configuration.WebUI.Port))
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

/*
handleWebSocket registers a new web socket client for receiving events and keeps reading from the connection until it
is closed, which then unregisters the client.
*/
func (s *Webserver) handleWebSocket(c *gin.Context) {
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	client := newWebSocketClient(ws)
	go client.writeLoop()
	s.websocketChannel <- WebSocketEvent{Client: client, Type: CONNECTED}
	defer func() {
		s.websocketChannel <- WebSocketEvent{Client: client, Type: DISCONNECTED}
		ws.Close()
	}()

	for {
		messageType, msg, err := ws.ReadMessage()
		if err != nil {
			break
		}

		if messageType == websocket.BinaryMessage || messageType == websocket.TextMessage {
			s.logger.Debug("received ws message", "msg", msg)
		}
	}
}

func (s *Webserver) getTrustModels(ctx *gin.Context) {
	ctx.IndentedJSON(http.StatusOK, s.tmts)
}

func (s *Webserver) getTrustSources(ctx *gin.Context) {
	ctx.IndentedJSON(http.StatusOK, s.trustSources)
}

func (s *Webserver) getTrustModel(ctx *gin.Context) {
	tmt, exists := s.tmts[ctx.Param("tmt-identifier")]
	if !exists {
		ctx.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND"})
	} else {
		ctx.IndentedJSON(http.StatusOK, tmt)
	}
}

func (s *Webserver) getInfo(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{"Version": version.Version, "Build": version.Build, "Configuration": s.tafContext.Configuration})
}

func (s *Webserver) SetManagers(managers manager.TafManagers) {

	for _, tmt := range managers.TMM.GetAllTMTs() {

		evidence := make(map[string]map[string]bool)
		for _, evidenceType := range tmt.EvidenceTypes() {
			if evidence[evidenceType.Source().String()] == nil {
				evidence[evidenceType.Source().String()] = make(map[string]bool)
			}
			evidence[evidenceType.Source().String()][evidenceType.String()] = true

			if s.trustSources[evidenceType.Source().String()] == nil {
				s.trustSources[evidenceType.Source().String()] = make(map[string]bool)
			}
			s.trustSources[evidenceType.Source().String()][evidenceType.String()] = true
		}

		s.tmts[tmt.Identifier()] = struct {
			Description   string
			Name          string
			Version       string
			EvidenceTypes map[string]map[string]bool
		}{
			Description:   tmt.Description(),
			Name:          tmt.TemplateName(),
			Version:       tmt.Version(),
			EvidenceTypes: evidence,
		}
	}
}

/*
frontendHandler serves the files of the frontend. As the frontend uses HTML5 history mode for its routes (e.g., /ui/tmis/),
all paths that do not match a file are answered with index.html, so that the frontend router can resolve them (e.g., when
reloading the page). Missing assets still result in a 404.
*/
func frontendHandler(frontendDir fs.FS) gin.HandlerFunc {
	index, err := fs.ReadFile(frontendDir, "index.html")
	if err != nil {
		panic(err)
	}
	fileServer := http.StripPrefix("/ui", http.FileServer(http.FS(frontendDir)))

	return func(c *gin.Context) {
		path := strings.TrimPrefix(c.Param("filepath"), "/")
		if info, err := fs.Stat(frontendDir, path); err == nil && !info.IsDir() {
			fileServer.ServeHTTP(c.Writer, c.Request)
		} else if strings.HasPrefix(path, "assets/") {
			c.Status(http.StatusNotFound)
		} else {
			c.Data(http.StatusOK, "text/html; charset=utf-8", index)
		}
	}
}
