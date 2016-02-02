package redisocket

import (
	"sync"

	"github.com/garyburd/redigo/redis"
)

type connpool struct {
	lock *sync.RWMutex
	pool map[*client]bool
}

func (cp *connpool) join(c *client) {

	redis.Dial("tcp", ":6379")
	cp.lock.Lock()
	cp.pool[c] = true
	cp.lock.Unlock()
	return
}

func (cp *connpool) remove(c *client) {
	cp.lock.Lock()
	delete(cp.pool, c)
	cp.lock.Unlock()
	return
}
