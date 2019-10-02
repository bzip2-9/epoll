package epoll

import (
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
	fd int
	// nthreads - default 1 (single thread), threads to dispatch event, not to wait
	nthreads uint
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

	ep := &Epoll{fd: fd, nthreads: nthreads}

	if nthreads > 0 {
		go ep.eventLoopSingleThread()
	}

	return ep, nil
}

// Add new connection to the epoll set - net.TCPConn
func (me *Epoll) Add(obj *UserObject, events int) {

	uptr := uint64((uintptr)(unsafe.Pointer(obj)))

	event := unix.EpollEvent{Events: uint32(events), Fd: int32(uptr & 0xFFFFFFFF), Pad: int32(uptr >> 32)}

	unix.EpollCtl(me.fd, unix.EPOLL_CTL_ADD, (*obj).GetFD(), &event)

}

// Del removes connection (net.TCPConn) from the epoll set
func (me *Epoll) Del(obj *UserObject) {
	unix.EpollCtl(me.fd, unix.EPOLL_CTL_DEL, (*obj).GetFD(), nil)
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

		for _, ev := range evList {

			u64 = uint64(ev.Fd) + (uint64(ev.Pad) << 32)
			uptr := uintptr(u64)
			obj := (*UserObject)(unsafe.Pointer(uptr))
			(*obj).Event(ev.Events)
		}
	}
}

// EventLoopNoThread - event loop used directly by library user to handle events, no threads
func (me *Epoll) EventLoopNoThread() {

	if me.nthreads == 0 {
		me.eventLoopSingleThread()
	}

}

// Destroy the epoll set
func (me *Epoll) Destroy() {
	unix.Close(me.fd)
}
