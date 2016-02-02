package redisocket

import (
	"fmt"
	"net/http"
	"os/exec"
	"regexp"
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

type message struct {
	to      string
	regex   *regexp.Regexp
	content []byte
}
type Listener interface {
	GetUUID() string
	GetTag() string
	Send(b []byte)
	Listen() error
}
type client struct {
	ws   *websocket.Conn
	send chan []byte
	uuid string
	tag  string
}

func NewClient(tag string, w http.ResponseWriter, r *http.Request) (c Listener, err error) {

	ws, err := Upgrader.Upgrade(w, r, nil)
	out, err := exec.Command("uuidgen").Output()
	if err != nil {
		return
	}
	uuid := fmt.Sprintf("%s", out)
	c = &client{
		ws:   ws,
		send: make(chan []byte, 4096),
		uuid: uuid,
		tag:  tag,
	}
	return
}

func (c *client) Send(b []byte) {
	c.send <- b
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
			_, _, err := c.ws.ReadMessage()
			if err != nil {
				errChan <- err
				close(errChan)
				return

			}
		}
	}()
	return errChan

}

func (c *client) Listen() (err error) {
	writeErr := c.writePump()
	readErr := c.readPump()
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
				if err := c.write(websocket.TextMessage, msg); err != nil {
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
