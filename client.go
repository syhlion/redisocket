package redisocket

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type client struct {
	ws   *websocket.Conn
	send chan []byte
	uuid string
	*sync.RWMutex
	MsgHandler
	app *app
}

func (c *client) Uuid() string {
	return c.uuid
}

func (c *client) Notify(data []byte) {
	c.send <- data
}

func (c *client) write(msgType int, data []byte) error {
	c.ws.SetWriteDeadline(time.Now().Add(writeWait))
	return c.ws.WriteMessage(msgType, data)
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

			err = c.Receive(c, data)
			if err != nil {
				errChan <- err
				break
			}
		}
	}()
	return errChan

}
func (c *client) Close() {
	c.app.UnsubAllEvent(c)
	c.app.leave <- c
	c.ws.Close()
	return
}

func (c *client) Listen() (err error) {
	writeErr := c.writePump()
	readErr := c.readPump()
	c.app.join <- c
	defer func() {
		c.Close()
	}()
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
				msg, err := c.Send(msg)
				if err != nil {
					continue
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
