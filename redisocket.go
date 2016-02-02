package redisocket

import (
	"net"
	"sync"
	"time"

	"github.com/garyburd/redigo/redis"
)

var conn redis.Conn

type EventCenter interface {
	On(event string, f func(data string)) error
	Emit(event string) error
}
type eventHandler struct {
	conn   redis.Conn
	events map[string]func(data string)
	*sync.RWMutex
}

func New(netConn net.Conn, readTimeout, writeTimeout time.Duration) {
	conn = redis.NewConn(netConn, readTimeout, writeTimeout)
}

func (e *eventHandler) On(event string, f func(data string)) (err error) {
	psc := redis.PubSubConn{e.conn}
	err = psc.Subscribe(event)
	e.Lock()
	e.events[event] = f
	e.Unlock()

	return
}
