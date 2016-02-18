package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/garyburd/redigo/redis"
	"github.com/syhlion/redisocket"
)

type ClientMessageHandler struct{ redisocket.App }

func (t *ClientMessageHandler) AfterReadStream(e redisocket.Subscriber, d []byte) (err error) {
	t.App.Notify("channel1", d)

	fmt.Println(string(d))
	return err

}
func (t *ClientMessageHandler) BeforeWriteStream(sub redisocket.Subscriber, data []byte) (d []byte, e error) {
	return data, nil
}

func main() {
	pool := redis.NewPool(func() (redis.Conn, error) {
		return redis.Dial("tcp", ":6379")
	}, 5)
	app := redisocket.NewApp(pool)

	go app.Listen()
	t := &ClientMessageHandler{app}

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {

		c, err := app.NewClient(t, w, r)
		if err != nil {
			log.Fatal("Client Connect Error")
			return
		}
		app.Subscribe("channel1", c)
		err = c.Listen()
		log.Println(err, "http point")
		return
	})
	http.HandleFunc("/ws2", func(w http.ResponseWriter, r *http.Request) {

		c, err := app.NewClient(t, w, r)
		if err != nil {
			log.Fatal("Client Connect Error")
			return
		}
		app.Subscribe("channel2", c)
		err = c.Listen()
		log.Println(err, "http point")
		return
	})

	log.Fatal(http.ListenAndServe(":8888", nil))
}
