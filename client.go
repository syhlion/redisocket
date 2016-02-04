package redisocket

import (
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

var Upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type Payload []byte

type MessageParser interface {
	Parse([]byte) Message
}
type Message struct {
	Event   string
	Payload Payload
}
type Client interface {
	GetUUID() string
	GetTag() string
	EventHandler
}
type client struct {
	ws   *websocket.Conn
	send chan Message
	uuid string
	tag  string
	*sync.RWMutex
	events map[string]func(data Payload) Payload
	MessageParser
	app *app
}

func (c *client) On(event string, f func(data Payload) Payload) (err error) {
	c.Lock()
	c.events[event] = f
	c.Unlock()
	return

}
func (c *client) Emit(event string, data Payload) (err error) {
	c.send <- Message{event, data}
	return
}

func (c *client) GetUUID() string {
	return c.uuid
}
func (c *client) GetTag() string {
	return c.tag
}

func (c *client) write(msgType int, msg []byte) error {
	c.ws.SetWriteDeadline(time.Now().Add(writeWait))
	return c.ws.WriteMessage(msgType, msg)
}

func (c *client) readPump() <-chan error {

	errChan := make(chan error)
	go func() {
		c.ws.SetReadLimit(maxMessageSize)
		c.ws.SetReadDeadline(time.Now().Add(pongWait))
		c.ws.SetPongHandler(func(string) error { c.ws.SetReadDeadline(time.Now().Add(pongWait)); return nil })
		for {
			msgType, data, err := c.ws.ReadMessage()
			if err != nil {
				errChan <- err
				close(errChan)
				return
			}
			if msgType != websocket.TextMessage {
				continue
			}
			m := c.Parse(data)
			c.send <- m
		}
	}()
	return errChan

}

func (c *client) Listen() (err error) {
	writeErr := c.writePump()
	readErr := c.readPump()
	c.app.join <- c
	select {
	case e := <-writeErr:
		return e
	case e := <-readErr:
		return e
	}
}

func (c *client) writePump() <-chan error {
	errChan := make(chan error)
	go func() {
		t := time.NewTicker(pingPeriod)
		defer func() {
			t.Stop()
		}()
		for {
			select {
			case msg, ok := <-c.send:
				if !ok {
					errChan <- c.write(websocket.CloseMessage, []byte{})
					close(errChan)
					return
				}
				var payload Payload
				if f, ok := c.events[msg.Event]; ok {
					payload = f(msg.Payload)
				} else {
					payload = msg.Payload
				}
				if err := c.write(websocket.TextMessage, payload); err != nil {
					errChan <- err
					close(errChan)
					return
				}

			case <-t.C:
				if err := c.write(websocket.PingMessage, []byte{}); err != nil {
					errChan <- err
					close(errChan)
					return
				}

			}
		}
	}()
	return errChan

}
