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
	connections map[int]*epollConnection
}

// Create creates new epoll set
func Create(nthreads uint) (*Epoll, error) {

	fd, err := unix.EpollCreate(10)
	if err != nil {
		return nil, err
	}

	// For now, only prepared to single thread
	nthreads = 1

	ep := &Epoll{fd: fd, mutexTid: 0, nthreads: nthreads, connections: make(map[int]*epollConnection)}

	go ep.events()

	return ep, nil
}

// Lock epoll set
func (me *Epoll) Lock() {

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

	tid := unix.Gettid()

	// Prevent one thread unblock mutex owned by another thread
	if tid != me.mutexTid {
		return
	}

	me.mutexTid = 0
	me.mutex.Unlock()
}

// Add new connection to the epoll set
func (me *Epoll) Add(connection *net.TCPConn, userData interface{}, events int, f Func) {
	oneshot := false
	if (events & EPOLLONESHOT) > 0 {
		oneshot = true
	}
	handle, _ := connection.SyscallConn()

	var fdSocket int
	handle.Control(func(fd uintptr) {
		ptr := (*int)(unsafe.Pointer(fd))
		fdSocket = *ptr
	})

	newConnection := &epollConnection{connection: connection, userData: userData, f: f, oneshot: oneshot, fd: fdSocket}

	me.Lock()
	me.connections[fdSocket] = newConnection

	uptr := uint64((uintptr)(unsafe.Pointer(newConnection)))

	event := unix.EpollEvent{Events: uint32(events), Fd: int32(uptr & 0xFFFF), Pad: int32(uptr >> 32)}

	unix.EpollCtl(me.fd, unix.EPOLL_CTL_ADD, fdSocket, &event)

	me.Unlock()
}

// Del removes connection from the epoll set
func (me *Epoll) Del(connection *net.TCPConn, close bool) {
	handle, _ := connection.SyscallConn()

	var fdSocket int

	me.Lock()

	handle.Control(func(fd uintptr) {
		ptr := (*int)(unsafe.Pointer(fd))
		fdSocket = *ptr
	})

	_, found := me.connections[fdSocket]

	if found {
		delete(me.connections, fdSocket)
	}
	unix.EpollCtl(me.fd, unix.EPOLL_CTL_DEL, fdSocket, nil)

	if close {
		connection.Close()
	}

	me.Unlock()
}

func (me *Epoll) events() {

	var evList []unix.EpollEvent
	var u64 uint64

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
				newConnection := (*epollConnection)(unsafe.Pointer(uptr))
				ret := newConnection.f(newConnection.connection, newConnection.userData)

				if newConnection.oneshot || ret == EpollStop {
					me.Del(newConnection.connection, false)
				}

				if ret == EpollDestroy {
					me.Del(newConnection.connection, true)
				}
			}
		}
		me.Unlock()
	}
}

// Destroy the epoll set
func (me *Epoll) Destroy() {
	me.Lock()
	me.connections = nil
	unix.Close(me.fd)
	me.Unlock()
}
