package epoll

// Func defines function signature to callback when there is events (net.TCPConn key)

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

// UserObject - Interface Objects from User must obey
type UserObject interface {
	GetFD() int
	Close()
	Event(events uint32) // same constants EPOLL*, ORed
}
