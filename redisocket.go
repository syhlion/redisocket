package redisocket

import (
	"encoding/json"
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
	EVENT_KICK     = "EVENT_KICK"
)

var conn redis.Conn
var Upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type Payload []byte

type Receiver interface {
	Receive(c Client, msg Message) error
}
type Message struct {
	From    string
	Event   string
	To      string
	Payload interface{}
}
type Client interface {
	GetUUID() string
	MsgHandler
}
type MsgCallback func(msg Message) (m Message, e error)

type MsgHandler interface {
	Listen() (err error)
	On(msgType string, f MsgCallback) error
	Emit(msg Message) error
}

type App interface {
	NewClient(m Receiver, w http.ResponseWriter, r *http.Request) (Client, error)
	Kick(uuid string)
	Count() int
	MsgHandler
}
type app struct {
	psc         *redis.PubSubConn
	conn        redis.Conn
	rpool       *redis.Pool
	events      map[string]MsgCallback
	clients     map[Client]bool
	clients_map map[string]Client
	join        chan Client
	leave       chan Client
	broadcast   chan Message
	kick        chan Message
	*sync.RWMutex
}

func NewApp(address string) App {
	pool := redis.NewPool(func() (redis.Conn, error) {

		return redis.Dial("tcp", address)

	}, 5)
	e := &app{
		rpool:       pool,
		psc:         &redis.PubSubConn{pool.Get()},
		events:      make(map[string]MsgCallback),
		clients:     make(map[Client]bool),
		clients_map: make(map[string]Client),
		RWMutex:     new(sync.RWMutex),
		join:        make(chan Client, 1024),
		leave:       make(chan Client, 1024),
		broadcast:   make(chan Message, 1024),
		kick:        make(chan Message, 1024),
	}
	e.On(EVENT_KICK, default_kick_callback)

	return e
}
func (e *app) NewClient(m Receiver, w http.ResponseWriter, r *http.Request) (c Client, err error) {
	ws, err := Upgrader.Upgrade(w, r, nil)
	out, err := exec.Command("uuidgen").Output()
	if err != nil {
		return
	}
	uuid := fmt.Sprintf("%s", out)
	c = &client{
		ws:       ws,
		send:     make(chan Message, 4096),
		events:   make(map[string]MsgCallback),
		RWMutex:  new(sync.RWMutex),
		uuid:     uuid,
		Receiver: m,
		app:      e,
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
				m := &Message{}
				err := json.Unmarshal(v.Data, m)
				if err != nil {
					continue
				}
				msg := *m
				if f, ok := e.events[msg.Event]; ok {
					m, err := f(msg)
					if err != nil {
						continue
					}
					msg = m
				}
				if msg.To == "" {

					e.broadcast <- msg
				} else {
					if c, ok := e.clients_map[msg.To]; ok {
						c.Emit(msg)
					}
				}

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
				e.clients_map[c.GetUUID()] = c
			case c := <-e.leave:
				delete(e.clients, c)
				delete(e.clients_map, c.GetUUID())
			case s := <-e.broadcast:
				for c, _ := range e.clients {
					c.Emit(s)
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
func (e *app) Kick(uuid string) {
	m := Message{
		To:    uuid,
		Event: "EVENT_KICK",
	}
	e.Emit(m)
}

func (e *app) Count() int {
	return len(e.clients)
}
func (e *app) On(event string, f MsgCallback) (err error) {
	err = e.psc.Subscribe(event)
	e.Lock()
	e.events[event] = f
	e.Unlock()

	return
}
func (e *app) Emit(msg Message) (err error) {
	m, err := json.Marshal(msg)
	if err != nil {
		return
	}

	_, err = e.rpool.Get().Do("PUBLISH", msg.Event, m)
	err = e.rpool.Get().Flush()
	return
}
