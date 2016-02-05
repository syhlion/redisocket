package main

import (
	"log"
	"net/http"

	"github.com/syhlion/redisocket"
)

type TestParser struct{ redisocket.App }

func (t *TestParser) Parse(e redisocket.EventHandler, data []byte) (err error) {
	//e.Emit("message", data)
	t.App.Emit("message", data)
	return err

}

func main() {
	app := redisocket.NewApp(":6379")
	err := app.On("message", func(data redisocket.Payload) (p redisocket.Payload, err error) {
		a := string(data) + "****bb"
		p = []byte(a)

		println(string(p))
		return
	})

	if err != nil {
		log.Fatal(err)
	}
	go app.Listen()
	t := &TestParser{app}

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {

		c, err := app.NewClient("test", t, w, r)
		if err != nil {
			log.Fatal("Client Connect Error")
			return
		}
		err = c.Listen()
		log.Println(err, "http point")
		return
	})

	http.HandleFunc("/kick", func(w http.ResponseWriter, r *http.Request) {

		app.Emit(redisocket.EVENT_KICK, []byte("kick"))
		w.WriteHeader(200)
		w.Write([]byte("ok"))
		return

	})

	log.Fatal(http.ListenAndServe(":8888", nil))
}
