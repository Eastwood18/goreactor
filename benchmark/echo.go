package main

import "goreaction"

type example struct {
}

func (s *example) OnConnect(c *goreaction.Connection) {}
func (s *example) OnMessage(c *goreaction.Connection, ctx interface{}, data []byte) (out interface{}) {
	return data
}

func (s *example) OnClose(c *goreaction.Connection) {
	//log.Error("onclose ")
}

func main() {

	handler := new(example)
	//var port int
	//var loops int

	//flag.IntVar(&port, "port", 1833, "server port")
	//flag.IntVar(&loops, "loops", -1, "num loops")
	//flag.Parse()

	s, err := goreaction.NewServer(handler)
	if err != nil {
		panic(err)
	}

	s.Start()
}
