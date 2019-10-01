package epoll

import (
	"net"
	"sync"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	// EPOLLIN - data available to read
	EPOLLIN int = unix.EPOLLIN
	// EPOLLOUT - writing is possible
	EPOLLOUT int = unix.EPOLLOUT
	// EPOLLET - Edge Triggered Event
	EPOLLET int = unix.EPOLLET
	// EPOLLPRI - Priority Event
	EPOLLPRI int = unix.EPOLLPRI
	// EPOLLHUP - fd was closed by the other side
	EPOLLHUP int = unix.EPOLLHUP
	// EPOLLONESHOT	- fd is removed from the epoll set after first event is triggered
	EPOLLONESHOT int = unix.EPOLLONESHOT
)

// Epoll represents an epoll set
type Epoll struct {
	fd       int
	mutex    sync.Mutex
	mutexTid int
	// nthreads - default 1 (single thread), threads to dispatch event, not to wait
	nthreads uint
	// connections - hold structures so GC doesn't destroy them
	connections map[int]*Connection
}

// Create creates new epoll set
// 0 threads => calling thread will take care of the event loop, no threads, no locks
// 1 thread => 1 thread is created to handle the event loop
// > 1 (N) threads => 1 thread is created to handle the event loop, N threads are
//     will be created to handle events in parallel  (TODO)
func Create(nthreads uint) (*Epoll, error) {

	fd, err := unix.EpollCreate(10)
	if err != nil {
		return nil, err
	}

	// For now, only prepared to single thread
	if nthreads > 1 {
		nthreads = 1
	}

	ep := &Epoll{fd: fd, mutexTid: 0, nthreads: nthreads, connections: make(map[int]*Connection)}

	if nthreads > 0 {
		go ep.eventLoopSingleThread()
	}

	return ep, nil
}

// Lock epoll set
func (me *Epoll) Lock() {

	if me.nthreads == 0 {
		return
	}
	tid := unix.Gettid()

	// Prevent self-blocking
	if tid == me.mutexTid {
		return
	}

	me.mutex.Lock()
	me.mutexTid = tid

}

// Unlock epoll set
func (me *Epoll) Unlock() {
	if me.nthreads == 0 {
		return
	}
	tid := unix.Gettid()

	// Prevent one thread unblock mutex owned by another thread
	if tid != me.mutexTid {
		return
	}

	me.mutexTid = 0
	me.mutex.Unlock()
}

// Add new connection to the epoll set - net.TCPConn
func (me *Epoll) Add(connection interface{}, customData interface{}, events int, f Func) bool {
	var tcp *net.TCPConn
	var ltcp *net.TCPListener
	var fdSocket int
	var kind connectionType
	var newConnection *Connection

	oneshot := false
	if (events & EPOLLONESHOT) > 0 {
		oneshot = true
	}

	switch connection.(type) {

	case int:
		fdSocket = connection.(int)
		kind = FD
		newConnection = &Connection{connection: fdSocket, customData: customData, f: f, oneshot: oneshot, fd: fdSocket, kind: kind}

	case *net.Conn, *net.TCPConn:
		tcp = (connection).(*net.TCPConn)
		handle, _ := tcp.SyscallConn()

		handle.Control(func(fd uintptr) {
			// ptr := (*int)(unsafe.Pointer(fd))
			// fdSocket = *ptr
			fdSocket = int(fd)
		})
		kind = CONN
		newConnection = &Connection{connection: tcp, customData: customData, f: f, oneshot: oneshot, fd: fdSocket, kind: kind}

	case *net.Listener, *net.TCPListener:
		ltcp = connection.(*net.TCPListener)
		handle, _ := ltcp.SyscallConn()

		handle.Control(func(fd uintptr) {
			// ptr := (*int)(unsafe.Pointer(fd))
			// fdSocket = *ptr
			fdSocket = int(fd)
		})
		kind = LISTENER
		newConnection = &Connection{connection: ltcp, customData: customData, f: f, oneshot: oneshot, fd: fdSocket, kind: kind}

	default:
		return false
	}

	me.Lock()
	me.connections[fdSocket] = newConnection

	uptr := uint64((uintptr)(unsafe.Pointer(newConnection)))

	event := unix.EpollEvent{Events: uint32(events), Fd: int32(uptr & 0xFFFFFFFF), Pad: int32(uptr >> 32)}

	unix.EpollCtl(me.fd, unix.EPOLL_CTL_ADD, fdSocket, &event)

	me.Unlock()

	return true
}

// Del removes connection (net.TCPConn) from the epoll set
func (me *Epoll) Del(connection interface{}, close bool) {
	var fdSocket int
	var tcp *net.TCPConn
	var ltcp *net.TCPListener
	var kind connectionType

	me.Lock()
	defer me.Unlock()

	switch connection.(type) {
	case int:
		fdSocket = connection.(int)
		kind = FD

	case *net.Conn:
		conn := connection.(*net.Conn)
		tcp = (*conn).(*net.TCPConn)
		handle, _ := tcp.SyscallConn()

		handle.Control(func(fd uintptr) {
			// ptr := (*int)(unsafe.Pointer(fd))
			// fdSocket = *ptr
			fdSocket = int(fd)
		})

	case *net.TCPConn:
		tcp = connection.(*net.TCPConn)
		handle, _ := tcp.SyscallConn()

		handle.Control(func(fd uintptr) {
			// ptr := (*int)(unsafe.Pointer(fd))
			// fdSocket = *ptr
			fdSocket = int(fd)
		})
		kind = CONN

	case *net.Listener:
		ltcp = connection.(*net.TCPListener)
		handle, _ := ltcp.SyscallConn()

		handle.Control(func(fd uintptr) {
			// ptr := (*int)(unsafe.Pointer(fd))
			// fdSocket = *ptr
			fdSocket = int(fd)
		})
		kind = LISTENER

	case *net.TCPListener:
		ltcp = connection.(*net.TCPListener)
		handle, _ := ltcp.SyscallConn()

		handle.Control(func(fd uintptr) {
			// ptr := (*int)(unsafe.Pointer(fd))
			// fdSocket = *ptr
			fdSocket = int(fd)
		})

		kind = LISTENER

	}

	me.Lock()

	_, found := me.connections[fdSocket]

	if found {
		delete(me.connections, fdSocket)
	}
	unix.EpollCtl(me.fd, unix.EPOLL_CTL_DEL, fdSocket, nil)

	if close {
		switch kind {
		case FD:
			unix.Close(fdSocket)

		case CONN:
			tcp.Close()

		case LISTENER:
			ltcp.Close()
		}
	}

	me.Unlock()

	tcp.Close()

}

// eventLoopSingleThread  => thread created when epoll.Create(1)
func (me *Epoll) eventLoopSingleThread() {

	var evList []unix.EpollEvent
	var u64 uint64

	evList = append(evList, unix.EpollEvent{})
	for {

		_, err := unix.EpollWait(me.fd, evList, -1)

		if err != nil {
			return
		}

		me.Lock()
		if me.nthreads == 1 {
			for _, ev := range evList {

				u64 = uint64(ev.Fd) + (uint64(ev.Pad) << 32)
				uptr := uintptr(u64)
				newConnection := (*Connection)(unsafe.Pointer(uptr))

				var ret FuncAction
				ret = newConnection.f(newConnection.connection, newConnection.customData)

				if newConnection.oneshot || ret == STOP || ret == DESTROY {
					me.Del(newConnection.connection, false)
				}

				if ret == DESTROY {
					newConnection.Close()
				}
			}
		}
		me.Unlock()
	}
}

// EventLoopNoThread - event loop used directly by library user to handle events, no threads
func (me *Epoll) EventLoopNoThread() {

	var evList []unix.EpollEvent
	evList = append(evList, unix.EpollEvent{})

	var u64 uint64

	for {

		_, err := unix.EpollWait(me.fd, evList, -1)

		if err != nil {
			return
		}

		for _, ev := range evList {

			u64 = uint64(ev.Fd) + (uint64(ev.Pad) << 32)
			uptr := uintptr(u64)
			newConnection := (*Connection)(unsafe.Pointer(uptr))

			var ret FuncAction
			ret = newConnection.f(newConnection.connection, newConnection.customData)

			if newConnection.oneshot || ret == STOP || ret == DESTROY {
				me.Del(newConnection.connection, false)
			}

			if ret == DESTROY {
				newConnection.Close()
			}
		}
	}

}

// Destroy the epoll set
func (me *Epoll) Destroy() {
	me.Lock()
	me.connections = nil
	unix.Close(me.fd)
	me.Unlock()
}
