package goreaction

//
//import (
//	"errors"
//	"github.com/libp2p/go-reuseport"
//	"golang.org/x/sys/unix"
//	"goreaction/eventloop"
//	"goreaction/poller"
//	"log"
//	"net"
//	"os"
//)
//
//type handleConnFunc func(fd int, sa unix.Sockaddr)
//
//type listener struct {
//	file     *os.File
//	fd       int
//	handleC  handleConnFunc
//	listener net.Listener
//	loop     *eventloop.EventLoop
//}
//
////func newListener(network, addr string, reusePort bool, handlerConn handleConnFunc) (*listener, error) {
////	var (
////		ls  net.Listener
////		err error
////	)
////	if reusePort {
////		// TODO rewrite reusePort
////		//ls, err = net.Listen(network, addr)
////		ls, err = reuseport.Listen(network, addr)
////		if err != nil {
////			return nil, err
////		}
////		//unix.SetsockoptInt(int(lsFd), unix.SOL_SOCKET, unix.SO_REUSEADDR, 1)
////
////	} else {
////		ls, err = net.Listen(network, addr)
////	}
////
////	if err != nil {
////		return nil, err
////	}
////
////	l, ok := ls.(*net.TCPListener)
////	if !ok {
////		return nil, errors.New("could not get file descriptor")
////	}
////
////	file, err := l.File()
////	if err != nil {
////		return nil, err
////	}
////	fd := int(file.Fd())
////	if err = unix.SetNonblock(fd, true); err != nil {
////		return nil, err
////	}
////	//if err = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEADDR, 1); err != nil {
////	//	return nil, err
////	//}
////
////	loop, err := eventloop.New()
////
////	if err != nil {
////		return nil, err
////	}
////
////	listener := &listener{
////		file:     file,
////		fd:       fd,
////		handleC:  handlerConn,
////		listener: ls,
////		loop:     loop,
////	}
////	if err = loop.AddSocketAndEnableRead(fd, listener); err != nil {
////		return nil, err
////	}
////
////	return listener, nil
////}
//
//func newListener(network, addr string, reusePort bool, handlerConn handleConnFunc) (*listener, error) {
//	var (
//		ls  net.Listener
//		err error
//	)
//	if reusePort {
//		ls, err = reuseport.Listen(network, addr)
//	} else {
//		ls, err = net.Listen(network, addr)
//	}
//	if err != nil {
//		return nil, err
//	}
//
//	l, ok := ls.(*net.TCPListener)
//	if !ok {
//		return nil, errors.New("could not get file descriptor")
//	}
//
//	file, err := l.File()
//	if err != nil {
//		return nil, err
//	}
//	fd := int(file.Fd())
//	if err = unix.SetNonblock(fd, true); err != nil {
//		return nil, err
//	}
//
//	loop, err := eventloop.New()
//
//	if err != nil {
//		return nil, err
//	}
//
//	listener := &listener{
//		file:     file,
//		fd:       fd,
//		handleC:  handlerConn,
//		listener: ls,
//		loop:     loop,
//	}
//	if err = loop.AddSocketAndEnableRead(fd, listener); err != nil {
//		return nil, err
//	}
//
//	return listener, nil
//}
//
//func (l *listener) Run() {
//	l.loop.Run()
//}
//
//func (l *listener) HandleEvent(fd int, events poller.Event) {
//	if events&poller.EventRead != 0 {
//		nfd, sa, err := unix.Accept(fd)
//		if err != nil {
//			if err != unix.EAGAIN {
//				log.Fatal("accept:", err)
//			}
//			return
//		}
//		if err := unix.SetNonblock(nfd, true); err != nil {
//			unix.Close(nfd)
//			log.Fatal("set nonblock:", err)
//			return
//		}
//
//		l.handleC(nfd, sa)
//	}
//}
//
//func (l *listener) Close() error {
//	return l.listener.Close()
//}
//
//func (l *listener) Stop() error {
//	return l.loop.Stop()
//}
