package redisocket

import (
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"sync"
	"time"

	"github.com/garyburd/redigo/redis"
)

var conn redis.Conn

type EventHandler interface {
	Listen() error
	On(event string, f func(data Payload) Payload) error
	Emit(event string, data Payload) error
}

type Center interface {
	NewClient(tag string, w http.ResponseWriter, r *http.Request) (Listener, error)
	EventHandler
}
type eventCenter struct {
	psc    *redis.PubSubConn
	conn   redis.Conn
	events map[string]func(data Payload) Payload
	*sync.RWMutex
}

func NewApp(netConn net.Conn, readTimeout, writeTimeout time.Duration) Center {
	conn = redis.NewConn(netConn, readTimeout, writeTimeout)
	e := &eventCenter{
		psc:     &redis.PubSubConn{conn},
		conn:    conn,
		events:  make(map[string]func(data Payload) Payload),
		RWMutex: new(sync.RWMutex),
	}
	return e
}
func (e *eventCenter) NewClient(tag string, w http.ResponseWriter, r *http.Request) (c Listener, err error) {
	ws, err := Upgrader.Upgrade(w, r, nil)
	out, err := exec.Command("uuidgen").Output()
	if err != nil {
		return
	}
	uuid := fmt.Sprintf("%s", out)
	c = &client{
		ws:   ws,
		send: make(chan Message, 4096),
		uuid: uuid,
		tag:  tag,
	}
	return
}

func (e *eventCenter) Listen() error {
	for {
		switch v := e.psc.Receive().(type) {
		case redis.Message:
			if f, ok := e.events[v.Channel]; ok {
				f(v.Data)
			}
		case error:
			return v
		}
	}
}

func (e *eventCenter) On(event string, f func(data Payload) Payload) (err error) {
	err = e.psc.Subscribe(event)
	e.Lock()
	e.events[event] = f
	e.Unlock()

	return
}
func (e *eventCenter) Emit(event string, data Payload) (err error) {
	err = e.conn.Send("PUBLISH", event, data)
	err = e.conn.Flush()
	return
}
