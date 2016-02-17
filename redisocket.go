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
)

var conn redis.Conn
var Upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

//MsgHandler Handle client i/o message
type MsgHandler interface {

	//AfterReadStream
	AfterReadStream(self Subscriber, data []byte) error
	//BeforeWriteStream
	BeforeWriteStream(self Subscriber, data []byte) ([]byte, error)
}

//Subscriber
type Subscriber interface {
	//Get uuid
	Uuid() string

	//Clients start listen. It's blocked
	Listen() error

	//Close clients connection
	Close()

	//When the subscribe subject update. App can Notify Subscriber
	Update(data []byte)
}

type App interface {
	// It client's Producer
	NewClient(m MsgHandler, w http.ResponseWriter, r *http.Request) (Subscriber, error)

	//A Subscriber can subscribe subject
	Subscribe(event string, c Subscriber) error

	//A Subscriber can  unsubscribe subject
	Unsubscribe(event string, c Subscriber) error

	//It can notify subscriber
	Notify(event string, data []byte) error

	//A subscriber can cancel all subscriptions
	UnsubscribeAll(c Subscriber)

	//App start listen. It's blocked
	Listen() error
}

//NewApp It's create a App
func NewApp(address string) App {
	pool := redis.NewPool(func() (redis.Conn, error) {

		return redis.Dial("tcp", address)

	}, 5)
	e := &app{
		rpool:       pool,
		psc:         &redis.PubSubConn{pool.Get()},
		RWMutex:     new(sync.RWMutex),
		events:      make(map[string]map[Subscriber]bool),
		subscribers: make(map[Subscriber]map[string]bool),
	}

	return e
}
func (e *app) NewClient(m MsgHandler, w http.ResponseWriter, r *http.Request) (c Subscriber, err error) {
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
	psc         *redis.PubSubConn
	conn        redis.Conn
	rpool       *redis.Pool
	events      map[string]map[Subscriber]bool
	subscribers map[Subscriber]map[string]bool
	*sync.RWMutex
}

func (a *app) Subscribe(event string, c Subscriber) (err error) {
	err = a.psc.Subscribe(event)
	if err != nil {
		return
	}
	a.Lock()

	//observer map
	if m, ok := a.subscribers[c]; ok {
		m[event] = true
	} else {
		events := make(map[string]bool)
		events[event] = true
		a.subscribers[c] = events
	}

	//event map
	if m, ok := a.events[event]; ok {
		m[c] = true
	} else {
		clients := make(map[Subscriber]bool)
		clients[c] = true
		a.events[event] = clients
	}
	a.Unlock()
	return
}
func (a *app) Unsubscribe(event string, c Subscriber) (err error) {
	a.Lock()

	//observer map
	if m, ok := a.subscribers[c]; ok {
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
func (a *app) UnsubscribeAll(c Subscriber) {
	a.Lock()
	if m, ok := a.subscribers[c]; ok {
		for e, _ := range m {
			delete(a.events[e], c)
		}
		delete(a.subscribers, c)
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

				for c, _ := range a.subscribers {
					a.UnsubscribeAll(c)
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
