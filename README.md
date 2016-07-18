# redisocket

Base on gorilla/websocket & garyburd/redigo

Implement By Observer pattern

*A new version,Plaease use this [redisocket.v2](https://www.github.com/syhlion/redisocket.v2)*

## Documention

* [API Reference](https://godoc.org/github.com/syhlion/redisocket)
* [Simple Example](https://github.com/syhlion/redisocket/blob/master/example/main.go)

## Install

`go get github.com/syhlion/redisocket`

## Usaged

``` go

type MessageHandle struct {
	app redisocket.App
	redisocket.Subscriber
}

func (c *MessageHandle) AfterReadStream(d []byte) (err error) {
	c.app.Notify("channel1", d)

	fmt.Println(string(d))
	return err

}
func (c *MessageHandle) BeforeWriteStream(data []byte) (d []byte, e error) {
	return data, nil
}

type Client struct {
    
	app redisocket.App
	redisocket.Subscriber
}

func (c *Client) Listen(){
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

		m:=MsgHandler{}
		sub, err := app.NewClient(m, w, r)
		if err != nil {
			log.Fatal("Client Connect Error")
			return
		}
		c:=&Client{app,sub}
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
```
