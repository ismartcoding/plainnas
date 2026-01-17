package eventbus

import "sync"

var eventbus Bus
var once sync.Once

func GetDefault() Bus {
	once.Do(func() {
		eventbus = New()
	})
	return eventbus
}
