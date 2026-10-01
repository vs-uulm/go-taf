package web

import (
	"time"

	"github.com/gorilla/websocket"
)

/*
WS_SEND_BUFFER defines how many messages can be queued for a single web socket client. A client that falls behind by
more than this amount is disconnected, so that a slow client cannot slow down the processing of events.
*/
const WS_SEND_BUFFER = 1024

/*
WS_WRITE_TIMEOUT defines how long writing a single message to a web socket client may take before the client is
considered dead.
*/
const WS_WRITE_TIMEOUT = 10 * time.Second

type WebSocketEventType int

const (
	CONNECTED    WebSocketEventType = 0
	DISCONNECTED WebSocketEventType = 1
)

type WebSocketEvent struct {
	Client *webSocketClient
	Type   WebSocketEventType
}

/*
webSocketClient wraps a web socket connection with its own queue of outgoing messages. Messages are written by a
dedicated go-routine (writeLoop), so that State.Handle never blocks on the network. The send channel is owned by
State.Handle, which is the only one sending to and closing it.
*/
type webSocketClient struct {
	conn *websocket.Conn
	send chan []byte
}

func newWebSocketClient(conn *websocket.Conn) *webSocketClient {
	return &webSocketClient{
		conn: conn,
		send: make(chan []byte, WS_SEND_BUFFER),
	}
}

/*
writeLoop writes all queued messages to the web socket until the send channel is closed. On a write error, the
connection is closed, which in turn makes the read loop of the connection fail and unregister the client.
*/
func (c *webSocketClient) writeLoop() {
	for msg := range c.send {
		c.conn.SetWriteDeadline(time.Now().Add(WS_WRITE_TIMEOUT))
		if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			c.conn.Close()
			return
		}
	}
}
