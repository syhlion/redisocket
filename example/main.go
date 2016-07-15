package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/garyburd/redigo/redis"
	"github.com/syhlion/redisocket"
)

type User struct {
	app redisocket.App
	redisocket.Subscriber
}

func (c *User) AfterReadStream(d []byte) (err error) {
	c.app.Notify("channel1", d)

	fmt.Println(string(d))
	return err

}
func (c *User) BeforeWriteStream(data []byte) (d []byte, e error) {
	return data, nil
}

func (c *User) Listen() (err error) {
	return c.Subscriber.Listen(c)

}

func main() {
	pool := redis.NewPool(func() (redis.Conn, error) {
		return redis.Dial("tcp", ":6379")
	}, 10)
	app := redisocket.NewApp(pool)

	err := make(chan error)
	go func() {
		err <- app.Listen()
	}()

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {

		sub, err := app.NewClient(w, r)
		if err != nil {
			log.Fatal("Client Connect Error")
			return
		}
		c := &User{app, sub}
		err = c.Listen()
		log.Println(err, "http point")
		return
	})

	go func() {
		err <- http.ListenAndServe(":8888", nil)
	}()
	select {
	case e := <-err:
		log.Println(e)
	}
}
