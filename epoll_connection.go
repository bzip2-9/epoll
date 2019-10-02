package epoll

// Func defines function signature to callback when there is events (net.TCPConn key)

// FuncAction - value returned by event callback
type FuncAction int

const (
	// CONTINUE - fd will remain in management
	OK FuncAction = 1
	// STOP - fd should be removed from management
	EXIT FuncAction = 2
)

// UserObject - Interface Objects from User must obey
type UserObject interface {
	GetFD() int
	GetFD0() int
	GetFD1() int
	Ptr() uintptr
	Close()
	Event(uo *UserObject, events uint32) FuncAction // same constants EPOLL*, ORed
}
