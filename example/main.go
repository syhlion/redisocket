package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/syhlion/redisocket"
)

type TestParser struct{ redisocket.App }

func (t *TestParser) Receive(e redisocket.Observer, d []byte) (err error) {
	t.App.Emit("channel1", d)

	fmt.Println(string(d))
	return err

}
func (t *TestParser) Send(data []byte) (d []byte, e error) {
	return data, nil
}

func main() {
	app := redisocket.NewApp(":6379")

	go app.Listen()
	t := &TestParser{app}

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {

		c, err := app.NewClient(t, w, r)
		if err != nil {
			log.Fatal("Client Connect Error")
			return
		}
		app.Sub("channel1", c)
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
		app.Sub("channel2", c)
		err = c.Listen()
		log.Println(err, "http point")
		return
	})

	log.Fatal(http.ListenAndServe(":8888", nil))
}
