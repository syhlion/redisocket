package redisocket

import "testing"

var a *app

var clients [3]*mclient

func init() {
	b := NewApp(":6379")
	a = b.(*app)
	go a.Listen()
	clients[0] = mockClient()
	clients[1] = mockClient()
	clients[2] = mockClient()
}

type mclient struct {
	receive []byte
}

func (m *mclient) Uuid() string {
	return "test"
}

func (m *mclient) Listen() (err error) {
	return
}

func (m *mclient) Close() {
	return
}

func (m *mclient) Update(data []byte) {
	m.receive = data
	return
}
func mockClient() *mclient {
	return &mclient{}
}

func subscribe(c Subscriber) {
	a.Subscribe("channel1", c)
	a.Subscribe("channel2", c)
	a.Subscribe("channel3", c)
}

func TestSubscribe(t *testing.T) {
	for _, c := range clients {
		subscribe(c)
	}
	if e, ok := a.events["channel1"]; ok {
		if _, ok := e[clients[0]]; !ok {
			t.Error("c1 no subscribe channel1")
		}
	} else {
		t.Error("c1 no subscribe channel1")
	}
	if e, ok := a.events["channel1"]; ok {
		if _, ok := e[clients[1]]; !ok {
			t.Error("c2 no subscribe channel1")
		}
	} else {
		t.Error("c2 no subscribe channel1")
	}
	if e, ok := a.events["channel2"]; ok {
		if _, ok := e[clients[0]]; !ok {
			t.Error("c1 no subscribe channel2")
		}
	} else {
		t.Error("c1 no subscribe channel2")
	}
	if e, ok := a.events["channel3"]; ok {
		if _, ok := e[clients[2]]; !ok {
			t.Error("c1 no subscribe channel3")
		}
	} else {
		t.Error("c1 no subscribe channel3")
	}
	if c, ok := a.subscribers[clients[2]]; ok {
		if _, ok := c["channel3"]; !ok {
			t.Error("c3 no subscribe channel3")
		}
	} else {
		t.Error("c3 no subscribe channel3")
	}
}

func TestUnsubscribe(t *testing.T) {
	a.Unsubscribe("channel1", clients[0])

	if e, ok := a.events["channel1"]; ok {
		if _, ok := e[clients[0]]; ok {
			t.Error("c1 no Unsubscribe channel1")
		}
	} else {

		t.Error("NO channel1")
	}
	a.Subscribe("channel1", clients[0])

}

func TestUnsubscribeAll(t *testing.T) {
	a.UnsubscribeAll(clients[0])
	if e, ok := a.events["channel2"]; ok {
		if _, ok := e[clients[0]]; ok {
			t.Error("c1 no Unsubscribe channel1")
		}
	} else {

		t.Error("No channel2")
	}
	a.Subscribe("channel1", clients[0])
	a.Subscribe("channel2", clients[0])
	a.Subscribe("channel3", clients[0])
}

func TestNotify(t *testing.T) {
	a.Notify("channel1", []byte("Hello"))

	if string(clients[0].receive) != "Hello" {
		t.Error("c1 no recevie notify")
	}
	if string(clients[1].receive) != "Hello" {
		t.Error("c2 no recevie notify")
	}
	if string(clients[2].receive) != "Hello" {
		t.Error("c3 no recevie notify")
	}
}
