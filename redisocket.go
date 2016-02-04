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
	Listen() (err error)
	On(event string, f func(data Payload) Payload) error
	Emit(event string, data Payload) error
}

type App interface {
	NewClient(tag string, m MessageParser, w http.ResponseWriter, r *http.Request) (Client, error)
	EventHandler
}
type app struct {
	psc       *redis.PubSubConn
	conn      redis.Conn
	events    map[string]func(data Payload) Payload
	clients   map[Client]bool
	join      chan Client
	leave     chan Client
	broadcast chan Message
	*sync.RWMutex
}

func NewApp(netConn net.Conn, readTimeout, writeTimeout time.Duration) App {
	conn = redis.NewConn(netConn, readTimeout, writeTimeout)
	e := &app{
		psc:       &redis.PubSubConn{conn},
		conn:      conn,
		events:    make(map[string]func(data Payload) Payload),
		RWMutex:   new(sync.RWMutex),
		join:      make(chan Client, 1024),
		leave:     make(chan Client, 1024),
		broadcast: make(chan Message, 1024),
	}
	return e
}
func (e *app) NewClient(tag string, m MessageParser, w http.ResponseWriter, r *http.Request) (c Client, err error) {
	ws, err := Upgrader.Upgrade(w, r, nil)
	out, err := exec.Command("uuidgen").Output()
	if err != nil {
		return
	}
	uuid := fmt.Sprintf("%s", out)
	c = &client{
		ws:            ws,
		send:          make(chan Message, 4096),
		uuid:          uuid,
		tag:           tag,
		MessageParser: m,
	}
	go c.Listen()
	e.join <- c
	return
}
func (e *app) listenRedis() <-chan error {
	errChan := make(chan error)
	go func() {
		for {
			switch v := e.psc.Receive().(type) {
			case redis.Message:
				var p Payload
				if f, ok := e.events[v.Channel]; ok {
					p = f(v.Data)
				} else {
					p = v.Data
				}
				e.broadcast <- Message{v.Channel, p}

			case error:
				errChan <- v
				close(errChan)
				break
			}
		}
	}()
	return errChan
}
func (e *app) listenClients() chan int {

	stop := make(chan int)
	go func() {
		for {
			select {
			case c := <-e.join:
				e.clients[c] = true
			case c := <-e.leave:
				delete(e.clients, c)
			case s := <-e.broadcast:
				for c, _ := range e.clients {
					c.Emit(s.Event, s.Payload)
				}
			case <-stop:
				for c, _ := range e.clients {
					e.leave <- c
				}
				close(stop)
				break
			}
		}
	}()
	return stop
}

func (e *app) Listen() error {
	redisErr := e.listenRedis()
	clientsStop := e.listenClients()
	defer func() {
		close(e.join)
		close(e.leave)
		close(e.broadcast)
	}()
	select {
	case e := <-redisErr:
		clientsStop <- 1
		return e
	}
}

func (e *app) On(event string, f func(data Payload) Payload) (err error) {
	err = e.psc.Subscribe(event)
	e.Lock()
	e.events[event] = f
	e.Unlock()

	return
}
func (e *app) Emit(event string, data Payload) (err error) {
	err = e.conn.Send("PUBLISH", event, data)
	err = e.conn.Flush()
	return
}
