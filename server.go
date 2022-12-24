/*
ssh-keep
   keep your ssh connection survive from network fluctuation or wifi switching

 Author: yurenchen@yeah.net
License: GPLv2
   Site: https://github.com/yurenchen000/ssh-keep
*/

package main

import (
	"bufio"
	"flag"
	"fmt"
	"net"
	"time"

	. "pkt"
)

const (
	listenAddr = "0.0.0.0:1234"
	serverAddr = "127.0.0.1:4321"
)

var flgListenAddr = flag.String("listen", listenAddr, "listen on addr")
var flgServerAddr = flag.String("server", serverAddr, "connect to server")

type Session struct {
	Sess

	connA net.Conn //Conn self is pointer inset, copy also work
	connB net.Conn
	readA chan []byte
	ctxA  chan int

	//ack for pkt
	timer     *time.Timer
	last_read time.Time
	last_send time.Time
}

func connA_new() net.Conn {
	connA, err := net.Dial("tcp", *flgServerAddr)
	if err != nil {
		fmt.Println("Error connecting to server:", err)
		return nil
	}

	fmt.Println("++ new connA to server established", connA)

	fmt.Printf("connA: %#v\n", connA)
	// defer connA.Close() //never close positively
	return connA
}

var sessMap map[uint32]*Session //must by pointer

func connA_get(sessId uint32) *Session {
	sess, ok := sessMap[sessId]
	if ok {
		fmt.Println("== found old session:", &sess, sess.connA, sess.connB, sess.Sess_id, sess.Recv_mid, sess.Self_mid)
		return sess
	}

	readA := make(chan []byte, 10)
	ctxA := make(chan int, 10) // chan to broadcast conn lost

	connA := connA_new()
	sess = &Session{
		connA: connA,
		readA: readA,
		ctxA:  ctxA,
		Sess: Sess{
			Pend_map: make(map[uint16][]byte),
		},
		timer: time.NewTimer(99 * time.Second),
	}
	sess.timer.Stop()
	sessMap[sessId] = sess
	fmt.Println("++ create new session:", &sess, connA)

	//keep suckA
	go suckA(bufio.NewReader(connA), sess.readA)

	return sess
}

func connB_init(connB net.Conn, sess *Session, need_mid uint16) { // struct must by pointer

	fmt.Println("connB old:", sess.connB)

	// close current connection if it exists
	if sess.connB != nil {
		fmt.Println("\n-- close old connB:", sess.connB)
		sess.connB.Close() //stop old readBWriteA
		close(sess.ctxA)   //stop old readAWriteB
		time.Sleep(time.Second)

		sess.ctxA = make(chan int, 10)
	}
	sess.connB = connB

	//send session ctx
	Println("send-sess:", "#", sess.Recv_mid, sess.Sess_id)
	sess.SendSession(connB)

	//re-send
	Println("re-send: ", &sess, need_mid, sess.Self_mid)
	sess.Pend_map.ReSend(need_mid, sess.Self_mid, connB)

	// start read/write routine for connA
	go readAWriteB(sess.readA, sess.connB, sess.ctxA, sess) //A -> B
	go readBWriteA(sess.connB, sess.connA, sess)            //B -> A
}

func connB_wait(connB net.Conn) {
	buf, _, rid := (&Sess{}).ReadMsg(connB) // not up recv_mid
	if rid < 0 {                            // drop conn
		connB.Close()
		return
	}
	// sessId := binary.BigEndian.Uint32(buf[:4])
	sessId, ok := DecSess(buf)
	if !ok {
		Println("DecSess err:", len(buf))
		connB.Close()
		return
	}

	//-------------------- A. connecter
	fmt.Println("-recv new B:", len(buf), buf, sessId, rid)
	sess := connA_get(sessId) //may block @ new conn
	Println("recv-sess:", "#", rid, sess.Self_mid)

	connB_init(connB, sess, uint16(rid)) //not block
}

func main() {
	flag.Parse()
	sessMap = make(map[uint32]*Session)

	//-------------------- B. listener
	listener := createListener(*flgListenAddr)
	if listener == nil {
		return
	}
	defer listener.Close()

	for {
		connB := acceptConnection(listener)
		if connB == nil {
			return
		}
		fmt.Println("\n++ new B Connection accepted:", connB)
		go connB_wait(connB)
	}
}

func createListener(addr string) net.Listener {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Println("Error creating listener:", err)
		return nil
	}
	return listener
}

func acceptConnection(listener net.Listener) net.Conn {
	conn, err := listener.Accept()
	if err != nil {
		fmt.Println("Error accepting connection:", err)
		return nil
	}
	return conn
}

func suckA(reader *bufio.Reader, readA chan<- []byte) {
	buf := make([]byte, 1024)
	for {
		n, err := reader.Read(buf)
		if err != nil {
			Println("######## Error reading from A conn:", err, "\n\n")
			return
		}
		tmp := make([]byte, n)
		copy(tmp, buf)
		readA <- tmp
	}
}

func readAWriteB(readA <-chan []byte, connB net.Conn, ctx <-chan int, sess *Session) {
	// read from conn1 and write to conn2
	fmt.Println("++ reading A writing B..", connB)
	for {
		select {
		case val, ok := <-ctx: //read OR closed
			Println("-- readAWriteB: ctxB close:", connB, val, ok)
			return
		case _ = <-sess.timer.C: //ack timeout: some msg no resp, so we send ack pkt
			if sess.last_send.After(sess.last_read) { //already ack
				continue
			}

			// send positive ack //no dat
			sess.Self_mid++
			fmt.Println("-- readAWriteB send ack", sess.Self_mid)
			err := sess.SendMsg([]byte{}, connB)
			if err != nil {
				fmt.Println("-- readAWriteB: writeB Error", connB, err)
				return
			}
			sess.last_send = time.Now()

			continue
		case input, ok := <-readA:
			sess.Self_mid++
			if !ok {
				Println("readA chan fail:", ok)
			}

			LogPkgInfo("- server send:", " \033[32m", MsgInfo(sess.Self_mid, sess.Recv_mid, len(input)), "\033[0m", connB)
			LogPkgBody("\n\033[32m", (BufMax(input, 50)), "\033[0m")

			err := sess.SendMsg(input, connB)
			if err != nil {
				fmt.Println("-- readAWriteB: writeB Error", connB, err)
				return
			}
			sess.last_send = time.Now()
		}

	}
}

func readBWriteA(connB, connA net.Conn, sess *Session) {
	fmt.Println("++ reading B writing A..", connB, connA)
	for {
		buf, mid, rid := sess.ReadMsg(connB)
		if mid == -1 {
			Println("-- reading B: Error")
			return
		}

		sess.last_read = time.Now() //time for positive ack
		sess.timer.Reset(time.Second * 2)

		if mid < int(sess.Recv_mid) && int(sess.Recv_mid)-mid < 3000 { //not loopback
			continue //drop dup pkt
		}
		sess.Recv_mid = uint16(mid) //up recv_mid
		// Println("= recv_msg:", sess.recv_mid, "len", len(buf))

		LogPkgInfo("- server recv:", "\033[33m", MsgInfo(uint16(mid), uint16(rid), len(buf)), "\033[0m ", connB)
		LogPkgBody("\n\033[33m", (BufMax(buf, 50)), "\033[0m")

		_, err := connA.Write(buf)
		if err != nil {
			fmt.Println("-- reading B writing A: write Error", connB, connA, err)
			return
		}
	}
}
