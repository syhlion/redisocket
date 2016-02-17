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
	EVENT_KICK     = "EVENT_KICK"
)

var conn redis.Conn
var Upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

//MsgHandler 每個連線的收到的訊息跟送出訊息的處理器
//
// Receive 接收每個從 websocket進來的訊息
//
// Send 寫入給每個 websocket stream
type MsgHandler interface {
	AfterReadStream(self Observer, data []byte) error
	BeforeWriteStream(self Observer, data []byte) ([]byte, error)
}

//Message
type Message struct {
	From    string
	Event   string
	To      string
	Payload interface{}
}

type Observer interface {
	Uuid() string
	Listen() error
	Close()
	Update(data []byte)
}

type App interface {
	NewClient(m MsgHandler, w http.ResponseWriter, r *http.Request) (Observer, error)
	Sub(event string, c Observer) error
	Unsub(event string, c Observer) error
	Notify(event string, data []byte) error
	UnsubAllEvent(c Observer)
	Listen() error
}

func NewApp(address string) App {
	pool := redis.NewPool(func() (redis.Conn, error) {

		return redis.Dial("tcp", address)

	}, 5)
	e := &app{
		rpool:     pool,
		psc:       &redis.PubSubConn{pool.Get()},
		RWMutex:   new(sync.RWMutex),
		events:    make(map[string]map[Observer]bool),
		observers: make(map[Observer]map[string]bool),
		join:      make(chan Observer, 1024),
		leave:     make(chan Observer, 1024),
	}

	return e
}
func (e *app) NewClient(m MsgHandler, w http.ResponseWriter, r *http.Request) (c Observer, err error) {
	ws, err := Upgrader.Upgrade(w, r, nil)
	out, err := exec.Command("uuidgen").Output()
	if err != nil {
		return
	}
	uuid := fmt.Sprintf("%s", out)
	c = &client{
		ws:         ws,
		send:       make(chan []byte, 4096),
		RWMutex:    new(sync.RWMutex),
		uuid:       uuid,
		MsgHandler: m,
		app:        e,
	}
	return
}

type app struct {
	psc       *redis.PubSubConn
	conn      redis.Conn
	rpool     *redis.Pool
	events    map[string]map[Observer]bool
	observers map[Observer]map[string]bool
	join      chan Observer
	leave     chan Observer
	*sync.RWMutex
}

func (a *app) Sub(event string, c Observer) (err error) {
	err = a.psc.Subscribe(event)
	if err != nil {
		return
	}
	a.Lock()

	//observer map
	if m, ok := a.observers[c]; ok {
		m[event] = true
	} else {
		events := make(map[string]bool)
		events[event] = true
		a.observers[c] = events
	}

	//event map
	if m, ok := a.events[event]; ok {
		m[c] = true
	} else {
		clients := make(map[Observer]bool)
		clients[c] = true
		a.events[event] = clients
	}
	a.Unlock()
	return
}
func (a *app) Unsub(event string, c Observer) (err error) {
	a.Lock()

	//observer map
	if m, ok := a.observers[c]; ok {
		delete(m, event)
	}
	//event map
	if m, ok := a.events[event]; ok {
		delete(m, c)
		if len(m) == 0 {
			err = a.psc.Unsubscribe(event)
			if err != nil {
				return
			}
		}
	}
	a.Unlock()

	return
}
func (a *app) UnsubAllEvent(c Observer) {
	a.Lock()
	if m, ok := a.observers[c]; ok {
		for e, _ := range m {
			delete(a.events[e], c)
		}
		delete(a.observers, c)
	}
	a.Unlock()
	return
}
func (a *app) listenRedis() <-chan error {

	errChan := make(chan error)
	go func() {
		for {
			switch v := a.psc.Receive().(type) {
			case redis.Message:
				a.RLock()
				clients := a.events[v.Channel]
				a.RUnlock()
				for c, _ := range clients {
					c.Update(v.Data)
				}

			case error:
				errChan <- v

				for c, _ := range a.observers {
					a.UnsubAllEvent(c)
					c.Close()
				}
				break
			}
		}
	}()
	return errChan
}
func (a *app) Listen() error {
	redisErr := a.listenRedis()
	select {
	case e := <-redisErr:
		return e
	}
}
func (e *app) Notify(event string, data []byte) (err error) {

	_, err = e.rpool.Get().Do("PUBLISH", event, data)
	err = e.rpool.Get().Flush()
	return
}
