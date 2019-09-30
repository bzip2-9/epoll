package epoll

import (
	"net"
)

// FuncConn defines function signature to callback when there is events (net.TCPConn key)
type FuncConn func(*net.TCPConn, interface{}) FuncAction

// FuncFD defines function signature to callback when there is events (fd key)
type FuncFD func(int, interface{}) FuncAction

// FuncAction - value returned by event callback
type FuncAction int

const (
	// CONTINUE - fd will remain in management
	CONTINUE FuncAction = 1
	// STOP - fd should be removed from management
	STOP FuncAction = 2
	// DESTROY - fd should be removed from management, and must be closed
	DESTROY FuncAction = 3
)

type epollConnection struct {
	connection *net.TCPConn
	fd         int
	customData interface{}
	oneshot    bool
	callFConn  bool
	fConn      FuncConn
	fFD        FuncFD
}

// Close connection
func (me *epollConnection) Close() {
	me.connection.Close()
	me.customData = nil
}
