package epoll

import (
	"net"
)

type EpollFuncAction int

const (
	EpollContinue EpollFuncAction = 1
	EpollStop     EpollFuncAction = 2
	EpollDestroy  EpollFuncAction = 3
)

type epollConnection struct {
	connection *net.TCPConn
	fd         int
	userData   interface{}
	oneshot    bool
	f          Func
}

func (me *epollConnection) Close() {
	me.connection.Close()
	me.userData = nil
}

// Func defines function signature to callback when there is events
type Func func(*net.TCPConn, interface{}) EpollFuncAction
