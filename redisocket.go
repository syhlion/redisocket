package redisocket

import (
	"fmt"
	"net/http"
	"os/exec"
	"sync"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
	EVENT_JOIN     = "EVENT_JOIN"
	EVENT_LEAVE    = "EVENT_LEAVE"
	EVENT_KICK     = "EVENT_KICK"
)

var conn redis.Conn
var Upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type Payload []byte

type MessageParser interface {
	Parse(EventHandler, []byte) error
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
type EventCallback func(data Payload) (p Payload, e error)

type EventHandler interface {
	Listen() (err error)
	On(event string, f EventCallback) error
	Emit(event string, data Payload) error
}

type App interface {
	NewClient(tag string, m MessageParser, w http.ResponseWriter, r *http.Request) (Client, error)
	Count() int
	EventHandler
}
type app struct {
	psc       *redis.PubSubConn
	conn      redis.Conn
	rpool     *redis.Pool
	events    map[string]EventCallback
	clients   map[Client]bool
	join      chan Client
	leave     chan Client
	broadcast chan Message
	*sync.RWMutex
}

func NewApp(address string) App {
	pool := redis.NewPool(func() (redis.Conn, error) {

		return redis.Dial("tcp", address)

	}, 5)
	e := &app{
		rpool:     pool,
		psc:       &redis.PubSubConn{pool.Get()},
		events:    make(map[string]EventCallback),
		clients:   make(map[Client]bool),
		RWMutex:   new(sync.RWMutex),
		join:      make(chan Client, 1024),
		leave:     make(chan Client, 1024),
		broadcast: make(chan Message, 1024),
	}
	e.On(EVENT_KICK, default_kick_callback)
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
		events:        make(map[string]EventCallback),
		RWMutex:       new(sync.RWMutex),
		uuid:          uuid,
		tag:           tag,
		MessageParser: m,
		app:           e,
	}
	c.On(EVENT_KICK, default_kick_callback)
	return
}
func (e *app) listenRedis() <-chan error {
	errChan := make(chan error)
	go func() {
		for {
			switch v := e.psc.Receive().(type) {
			case redis.Message:
				var payload Payload
				if f, ok := e.events[v.Channel]; ok {
					p, err := f(v.Data)
					if err != nil {
						continue
					}
					payload = p
				} else {
					payload = v.Data
				}
				e.broadcast <- Message{v.Channel, payload}

			case error:
				errChan <- v
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
				break
			}
		}
	}()
	return stop
}

func (e *app) Listen() error {
	redisErr := e.listenRedis()
	clientsStop := e.listenClients()
	select {
	case e := <-redisErr:
		clientsStop <- 1
		return e
	}
}

func (e *app) Count() int {
	return len(e.clients)
}
func (e *app) On(event string, f EventCallback) (err error) {
	err = e.psc.Subscribe(event)
	e.Lock()
	e.events[event] = f
	e.Unlock()

	return
}
func (e *app) Emit(event string, data Payload) (err error) {
	_, err = e.rpool.Get().Do("PUBLISH", event, string(data))
	err = e.rpool.Get().Flush()
	return
}
