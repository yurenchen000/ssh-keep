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
	"io"
	"math/rand"
	"net"
	"os"
	"time"

	. "pkt"
)

var build_version string
var ServerAddr = "127.0.0.1:1234"
var flgServerAddr = flag.String("server", ServerAddr, "connect to server")
var flgVersion = flag.Bool("version", false, "current version: "+build_version)

var conn_gone = false
var last_read time.Time = time.Now() //last read from server
var last_unack time.Time

const AckTimeOut = 3

type Session struct {
	Sess

	connB net.Conn
	readI chan []byte
	ctxI  chan int
}

func main() {
	flag.Parse()
	if *flgVersion {
		Println("version:", build_version)
		return
	}

	// pseudo random number
	rand.Seed(time.Now().UnixNano())

	stdinReader := bufio.NewReader(os.Stdin)
	stdoutWriter := bufio.NewWriter(os.Stdout)

	sess := Session{
		Sess: Sess{
			Sess_id:  rand.Uint32(),
			Pend_map: make(map[uint16][]byte),
		},
		ctxI:  make(chan int, 10),    // chan to broadcast conn lost,
		readI: make(chan []byte, 10), // chan to send msg,
	}

	// Connect to the server
	conn, err := connect(*flgServerAddr, stdoutWriter, &sess)
	if err != nil {
		Println("Error connecting to server:", err)
		return
	}
	defer conn.Close()

	//send sess ctx
	Println("send-sess:", "#", sess.Recv_mid, sess.Sess_id)
	sess.SendSession(conn)

	//wait sess
	_, _, rid := sess.ReadMsg(conn) // not up recv_mid
	if rid == -1 {
		Println("-- reading from Server: Error")
		return
	}
	Println("recv-sess:", "#", rid, sess.Self_mid)

	go readFromServer(conn, stdoutWriter, &sess)
	go readFromStdin(sess.readI, conn, sess.ctxI, &sess)
	suckStdin(stdinReader, &sess)

	// select {}
}

// Try to connect to the server
func connect(host string, stdoutWriter *bufio.Writer, sess *Session) (net.Conn, error) {
	conn, err := net.Dial("tcp", host)
	if err != nil {
		return nil, err
	}

	// Start a goroutine to monitor the connection
	go func() {
		for {
			// Check if the connection is still alive
			now := time.Now()
			wait_time := now.Sub(last_unack)
			if conn_gone ||
				last_unack.After(last_read) &&
					wait_time > time.Second*AckTimeOut {
				// The connection is lost, try to reconnect

				Println("\n-- conn lost, trying re-conn...", conn, wait_time.Round(time.Millisecond), conn_gone)
				close(sess.ctxI) //notify readFromStdin stop
				sess.ctxI = make(chan int, 10)

				for {
					newConn, err := net.Dial("tcp", host)
					if err == nil {
						conn_gone = false
						Println("++ conn re-established", conn)

						// Replace the old connection with the new one
						conn.Close()
						conn = newConn

						// send-sess ctx
						Println("send-sess:", "#", sess.Recv_mid, sess.Sess_id)
						sess.SendSession(conn)

						// wait-sess
						_, _, rid := sess.ReadMsg(conn) // not up recv_mid
						if rid == -1 {
							Println("-- reading from Server: Error")
							conn_gone = true
							time.Sleep(time.Second * 3)
							// continue //re-conn
							break
						}
						last_read = time.Now()

						Println("recv-sess:", "#", rid, sess.Self_mid)
						Println("re-send: ", &sess, rid, sess.Self_mid)
						ret := sess.Pend_map.ReSend(uint16(rid), sess.Self_mid, conn)
						if ret == -1 { //lost mid
							Println("#### exit, lost mid:")
							os.Exit(1)
						} else if ret == -2 {
							Println("#### lost conn? break")
							break
						}

						// Restart the goroutine with the new connection
						go readFromServer(conn, stdoutWriter, sess)
						go readFromStdin(sess.readI, conn, sess.ctxI, sess)
						Println("reconn wait:", 20)
						time.Sleep(time.Second * 20)
						break
					}

					// Wait a moment before trying to reconnect again
					time.Sleep(time.Second)
				}
			}
			time.Sleep(time.Second)
		}
	}()

	Println("++ Connection to server established", conn)
	return conn, nil
}

func readMsg(reader net.Conn) (out []byte, mid int) {
	buf := make([]byte, 4)
	n, err := io.ReadFull(reader, buf)
	Println("readMsgH:", err, buf)
	if err != nil || n != 4 {
		return nil, -1
	}

	pkt := DecHead(buf)
	mid = int(pkt.Mid)
	out = make([]byte, pkt.Len)
	n, err = io.ReadFull(reader, out)
	Println("readMsgB:", err, out)

	if err != nil {
		return nil, -1
	}
	return out, mid
}

func readFromServer(reader net.Conn, writer *bufio.Writer, sess *Session) {
	Println("++ reading from Server..")
	for {
		// Read the response from the server
		buf, mid, rid := sess.ReadMsg(reader)
		if mid == -1 {
			Println("-- reading from Server: Error")
			return
		}

		if mid <= int(sess.Recv_mid) && int(sess.Recv_mid)-mid < 3000 { //not loopback
			continue //drop dup pkt
		}
		sess.Recv_mid = uint16(mid)
		// Println("= recv_msg:", recv_mid, "len", len(buf))

		last_read = time.Now()
		LogPkgInfo("- client recv:", " \033[32m", MsgInfo(uint16(mid), uint16(rid), len(buf)), "\033[0m")
		LogPkgBody("\n\033[32m", (BufMax(buf, 50)), "\033[0m")

		err := writeToStdout(buf, writer)
		if err != nil {
			Println("-- writing to stdout: Error", err)
			return
		}
	}
}

func suckStdin(reader *bufio.Reader, sess *Session) {
	buf := make([]byte, 1024)
	for {
		// Read from os.Stdin
		n, err := reader.Read(buf)
		if err != nil || n == 0 {
			Println("Error reading stdin:", err, n)
			return
		}
		tmp := make([]byte, n)
		// Println("arr len:", len(tmp), cap(tmp))
		copy(tmp, buf)
		sess.readI <- tmp
	}
}

func readFromStdin(read <-chan []byte, writer net.Conn, ctx <-chan int, sess *Session) {
	Println("++ reading from Stdin chan..")
	for {
		select {
		case val, ok := <-ctx: //read OR closed
			Println("-- reading from Stdin chan: close:", val, ok)
			return
		case input, ok := <-read:
			sess.Self_mid++
			if !ok {
				Println("readA chan fail:", ok)
			}

			now := time.Now()
			if last_unack.Before(last_read) { //had ack
				last_unack = now //not ack
			}

			LogPkgInfo("- client send:", "\033[33m", MsgInfo(sess.Self_mid, sess.Recv_mid, len(input)), "\033[0m", now.Sub(last_unack).Round(time.Millisecond)) // > 10s maybe lost // should < 0
			LogPkgBody("\n\033[33m", BufMax(input, 50), "\033[0m")

			// Send the input to the server
			err := sess.SendMsg(input, writer)
			if err != nil {
				conn_gone = true
				Println("-- Error writing to server:", err)
				return
			}
		}

	}
}

func writeToStdout(output []byte, writer *bufio.Writer) error {
	_, err := writer.Write(output)
	if err != nil {
		return err
	}
	return writer.Flush()
}
