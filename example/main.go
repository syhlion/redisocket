package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/syhlion/redisocket"
)

type TestParser struct{ redisocket.App }

func (t *TestParser) Receive(e redisocket.Client, msg redisocket.Message) (err error) {
	//e.Emit("message", data)
	t.App.Emit(msg)
	fmt.Println(msg)
	return err

}

func main() {
	app := redisocket.NewApp(":6379")
	err := app.On("message", func(msg redisocket.Message) (m redisocket.Message, err error) {
		a := msg.Payload.(string) + "****bb"

		println(string(a))
		return
	})

	if err != nil {
		log.Fatal(err)
	}
	go app.Listen()
	t := &TestParser{app}

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {

		c, err := app.NewClient(t, w, r)
		if err != nil {
			log.Fatal("Client Connect Error")
			return
		}
		err = c.Listen()
		log.Println(err, "http point")
		return
	})

	http.HandleFunc("/kick", func(w http.ResponseWriter, r *http.Request) {

		m := redisocket.Message{
			From:    "admin",
			To:      "",
			Payload: "kick",
			Event:   redisocket.EVENT_KICK,
		}
		app.Emit(m)
		w.WriteHeader(200)
		w.Write([]byte("ok"))
		return

	})

	log.Fatal(http.ListenAndServe(":8888", nil))
}
