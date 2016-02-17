package redisocket_test

import (
	"log"
	"net/http"

	"github.com/syhlion/redisocket"
)

func Example() {

	app := redisocket.NewApp(":6379")

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

	log.Fatal(http.ListenAndServe(":8888", nil))
}
