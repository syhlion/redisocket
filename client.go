package redisocket

import (
	"errors"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type client struct {
	ws   *websocket.Conn
	send chan Message
	uuid string
	tag  string
	*sync.RWMutex
	events map[string]EventCallback
	MessageParser
	app *app
}

func default_kick_callback(data Payload) (p Payload, err error) {

	return data, err
}

func (c *client) On(event string, f EventCallback) (err error) {
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
			err = c.Parse(c, data)
			if err != nil {
				errChan <- err
				break
			}
		}
	}()
	return errChan

}

func (c *client) Listen() (err error) {
	writeErr := c.writePump()
	readErr := c.readPump()
	c.app.join <- c
	defer func() {
		c.ws.Close()
		c.app.leave <- c
	}()
	//	err = c.app.conn.Send("SADD", c.tag, c.uuid)
	//	if err != nil {
	//		return
	//	}
	//	defer func() {
	//		c.app.conn.Send("SREM", c.tag, c.uuid)
	//	}()
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
					p, err := f(msg.Payload)
					if err != nil {
						continue
					}
					payload = p
				} else {
					payload = msg.Payload
				}
				if err := c.write(websocket.TextMessage, payload); err != nil {
					errChan <- err
					close(errChan)
					return
				}
				if msg.Event == EVENT_KICK {
					errChan <- errors.New(EVENT_KICK)
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
