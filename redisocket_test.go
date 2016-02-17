package redisocket

import "testing"

var a *app

var clients [3]Subscriber

func init() {
	b := NewApp(":6379")
	a = b.(*app)
	go a.Listen()
	clients[0] = mockClient()
	clients[1] = mockClient()
	clients[2] = mockClient()
}

type mclient struct {
	receivecCount int
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
	m.receivecCount++
	return
}
func mockClient() Subscriber {
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

}

func TestUnsubscribeAll(t *testing.T) {
}

func TestNotify(t *testing.T) {
}
