package main

import (
	"log"
	"net"
	"time"
)

// FeType define the type of event
type FeType int8
type TeType int8

const (
	AE_READABLE FeType = 1
	AE_WRITABLE FeType = 2

	AE_NORNAL TeType = 1
	AE_ONCE   TeType = 2
)

type FileProc func(loop *AeEventLoop, conn *net.TCPConn, extra any)
type TimeProc func(loop *AeEventLoop, id int, extra any)

// AcceptHandler blocked to wait for connection
func AcceptHandler() {
	err := server.listener.SetDeadline(time.Now().Add(time.Millisecond * 12))
	if err != nil {
		log.Println("set listener error: ", err)
		return
	}
	for {
		conn, err := server.listener.Accept()
		if err != nil {
			opErr := err.(*net.OpError)
			if opErr.Timeout() {
				return
			}
			//  if isn't time out error
			log.Println("accept error: ", err)
			return
		}
		tcpConn, ok := conn.(*net.TCPConn)
		if !ok { //不是tcp连接
			return
		}
		client := NewClient(tcpConn)
		server.clients[getConnFd(tcpConn)] = client
		// the connection can be read
		server.aeloop.AddFileEvent(tcpConn, AE_READABLE, ReadQueryFromClient, client)
	}

}

// ReadQueryFromClient the 'extra' should store the client
// TODO:optimized the reading implementation
var ReadQueryFromClient FileProc = func(loop *AeEventLoop, conn *net.TCPConn, extra any) {
	client := extra.(*GedisClient)
	//expand query buffer's capacity if it is less than the max command capacity，
	if len(client.queryBuf)-client.queryLen < GEDIS_MAX_CMD_BUF {
		client.queryBuf = append(client.queryBuf, make([]byte, GEDIS_MAX_CMD_BUF)...)
	}
	// set read deadline with 5 ms
	if err := conn.SetReadDeadline(time.Now().Add(time.Millisecond * 5)); err != nil {
		log.Printf("client %v set read dead line error: %v\n", conn, err)
		freeClient(client)
		return
	}

	n, err := conn.Read(client.queryBuf[client.queryLen:])
	if err != nil {
		opErr := err.(*net.OpError)
		if opErr.Timeout() { //expected read time out error
			return
		}
		log.Printf("client %v read error: %v", conn, err)
		freeClient(client)
		return
	}
	client.queryLen += n

	err = client.ProcessQueryBuf()
	if err != nil {
		log.Printf("process query buf err:%v", err)
		freeClient(client)
		return
	}
}

var ServerCron TimeProc = func(loop *AeEventLoop, id int, extra any) {
	// TODO: 执行对键值的随机检查
}

var SendReplyToClient FileProc = func(loop *AeEventLoop, conn *net.TCPConn, extra any) {
	client := extra.(*GedisClient)
	for client.reply.Length() > 0 {
		rep := client.reply.First()
		buf := []byte(rep.Val.Val_.(string))
		bufLen := len(buf)
		if client.sentLen < bufLen {
			n, err := conn.Write(buf[client.sentLen:])
			if err != nil {
				log.Println("sent reply error: ", err)
				freeClient(client)
				return
			}
			client.sentLen += n
			if client.sentLen == bufLen {
				client.reply.DelNode(rep)
				client.sentLen = 0
			} else {
				break
			}
		}
	}
	if client.reply.Length() == 0 { //finish write
		client.sentLen = 0
		loop.RemoveFileEvent(conn, AE_WRITABLE)
	}
}

// AeFileEvent deal with net i/o between server and client
type AeFileEvent struct {
	connection *net.TCPConn
	mask       FeType //文件事件类型
	proc       FileProc
	extra      interface{}
}

type AeTimeEvent struct {
	id       int
	mask     TeType
	when     int64 //when the time event will happen
	interval int64
	proc     TimeProc
	extra    interface{}
	next     *AeTimeEvent
}

type AeEventLoop struct {
	FileEvents      map[int]*AeFileEvent
	TimeEventsHead  *AeTimeEvent
	nextTimeEventID int
	stopped         bool
}

func NewAeEventLoop() *AeEventLoop {
	return &AeEventLoop{
		FileEvents:      make(map[int]*AeFileEvent),
		nextTimeEventID: 1,
		stopped:         false,
	}
}

// determine the index via connection and event type
func getFeKey(conn *net.TCPConn, mask FeType) int {
	fd := getConnFd(conn)
	if mask == AE_READABLE {
		return fd
	}
	return -1 * fd
}

// get file descriptor by connection
func getConnFd(conn *net.TCPConn) int {
	rawConn, err := conn.SyscallConn()
	if err != nil {
		log.Println("get raw connection error: ", err)
		return 0
	}
	var FD int
	err = rawConn.Control(func(fd uintptr) {
		FD = int(fd)
	})
	if err != nil {
		log.Println("executing raw connection's custom function error: ", err)
		return 0
	}
	return FD
}

func (loop *AeEventLoop) AddFileEvent(conn *net.TCPConn, mask FeType, proc FileProc, extra interface{}) {
	fe := AeFileEvent{
		connection: conn,
		mask:       mask,
		proc:       proc,
		extra:      extra,
	}
	loop.FileEvents[getFeKey(conn, mask)] = &fe
}

func (loop *AeEventLoop) RemoveFileEvent(conn *net.TCPConn, mask FeType) {
	delete(loop.FileEvents, getFeKey(conn, mask))
}

func GetTimeMs() int64 {
	return time.Now().UnixMilli()
}

// AddTimeEvent insert at the head of the linked list
func (loop *AeEventLoop) AddTimeEvent(mask TeType, interval int64, proc TimeProc, extra interface{}) int {
	nextID := loop.nextTimeEventID
	loop.nextTimeEventID++
	te := AeTimeEvent{
		id:       nextID,
		mask:     mask,
		interval: interval,
		when:     GetTimeMs() + interval,
		proc:     proc,
		extra:    extra,
		next:     loop.TimeEventsHead,
	}
	loop.TimeEventsHead = &te
	return nextID
}

func (loop *AeEventLoop) RemoveTimeEvent(id int) {
	p := loop.TimeEventsHead
	var pre *AeTimeEvent
	for p != nil {
		if p.id == id {
			if pre == nil {
				loop.TimeEventsHead = p.next
			} else {
				pre.next = p.next
			}
			p.next = nil
			break
		}
		pre = p
		p = p.next
	}
}

func (loop *AeEventLoop) AeProcess() {

	for _, fe := range loop.FileEvents {
		fe.proc(loop, fe.connection, fe.extra)
	}

	p := loop.TimeEventsHead
	now := GetTimeMs()
	for p != nil {
		if p.when <= now {
			p.proc(loop, p.id, p.extra)
			if p.mask == AE_ONCE {
				loop.RemoveTimeEvent(p.id)
			} else if p.mask == AE_NORNAL {
				p.when = GetTimeMs() + p.interval //set next trigger time
			}
		}
		p = p.next
	}
}

func (loop *AeEventLoop) AeMain(accept func()) {
	for !loop.stopped {
		accept()
		loop.AeProcess()
	}
}
