package epoll

import (
	"net"

	"golang.org/x/sys/unix"
)

// Func defines function signature to callback when there is events (net.TCPConn key)
type Func func(interface{}, interface{}) FuncAction

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

type connectionType int

const (
	// FD - file descriptor
	FD connectionType = 1
	// CONN net.conn
	CONN connectionType = 2
	// LISTENER net.Listener
	LISTENER connectionType = 3
)

type Connection struct {
	connection interface{}
	fd         int
	customData interface{}
	oneshot    bool
	kind       connectionType
	f          Func
}

// Close connection
func (me *Connection) Close() {

	switch me.kind {
	case FD:
		unix.Close(me.fd)

	case CONN:
		tcp := me.connection.(*net.TCPConn)
		tcp.Close()

	case LISTENER:
		ltcp := me.connection.(*net.TCPListener)
		ltcp.Close()
	}
	me.customData = nil
}
