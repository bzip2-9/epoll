# epoll
Golang Epoll Support - Linux

Getting the library:  go get -u github.com/bzip2-9/epoll

<pre><code>
Usage:

import (
  "net"
  "bzip2-9/epoll"
  )


func handleConnection (conn *net.Conn, userData interface{}) {

}

func newConnection (conn *net.Conn, userData interface{}) {

    client, _ := conn.AcceptTCP()


}

func main () {
  sockserver, _ := ListenTCP("127.0.0.1:8080",nil)

  ep := epoll.Create(1)

  ep.Add(sockserver,newConnection)

  ....
}

</code></pre>
