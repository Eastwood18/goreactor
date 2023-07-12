package eventloop

import (
	"log"
	"testing"
	"time"
	"unsafe"
)

func TestEventLoop_RunLoop(t *testing.T) {
	el, err := New()
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 10; i++ {
		go func() {
			el.QueueInLoop(func() {
				log.Println("run in loop")
			})
		}()
	}

	go func() {
		time.Sleep(time.Second)
		if err := el.Stop(); err != nil {
			panic(err)
		}
	}()

	el.Run()
}

func TestEventLoopSize(t *testing.T) {
	t.Log(unsafe.Sizeof(eventLoopLocal{}))
	t.Log(unsafe.Sizeof(EventLoop{}))
}
