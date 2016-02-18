# Redisocket
[![Build Status](https://travis-ci.org/syhlion/redisocket.svg?branch=master)](https://travis-ci.org/syhlion/redisocket)

Base on gorilla/websocket & garyburd/redigo

Implement By Observer pattern

## Install

`go get github.com/syhlion/redisocket`

## Useged

``` go
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
	app := redisocket.NewApp(":6379")

	go app.Listen()
	t := &ClientMessageHandler{app}

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {

		c, err := app.NewClient(t, w, r)
		if err != nil {
			log.Fatal("Client Connect Error")
			return
		}

		// It can subscribe many subject 
		//ex app.Subscribe("channel2",c)...etc
		app.Subscribe("channel1", c)
		err = c.Listen()
		log.Println(err, "http point")
		return
	})

	log.Fatal(http.ListenAndServe(":8888", nil))
}
```
