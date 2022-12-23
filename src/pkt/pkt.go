// pkt
package pkt

// package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"sort"
	"sync"
	"unsafe"
	// "github.com/davecgh/go-spew/spew"
)

//---------- util
func Println(a ...any) (n int, err error) {
	return fmt.Fprintln(os.Stderr, a...)
}
func DumbLog(a ...any) (n int, err error) {
	return 0, nil
}

var LogMsg = DumbLog

// var LogMsg = Println
// var LogSessMsg = Println

var LogSessMsg = DumbLog
var LogReMsg = DumbLog
var LogPkgInfo = Println
var LogPkgBody = DumbLog
var LogPend = DumbLog

//----------
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func BufMax(buf []byte, max int) []byte {
	return buf[:min(len(buf), max)]
}
func MsgInfo(Mid, Rid uint16, Len int) string {
	return fmt.Sprintf("#%-4d @%-4d (%-4d", Mid, Rid, Len)
}
func Keys(m PendMap) []uint16 {
	keys := make([]uint16, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func main() {
	fmt.Println("Hello World!")
	fmt.Println("size_MsgH:", size_MsgH)
	// msg := MsgH{12, 34}
	msg := MsgH{12, 34, 56}
	// msg := MsgH{1234, 5678}
	buf := EncHead(msg)
	fmt.Println("msg:", msg)
	fmt.Println("buf:", buf)

	m2 := DecHead(buf)
	fmt.Println("out:", m2)

	msg2 := MsgS{
		// MsgH.Len: 4,
		MsgH: MsgH{Len: 4, Mid: 12},
		// Len: 4,
		// Mid: 4,
		Sid: 5678,
	}
	fmt.Println("ses2:", msg2)

	out2 := EncSess(msg2)
	fmt.Println("ses2:", out2)

	// fmt.Println("spew:")
	// spew.Dump(msg2)
	// fmt.Println("spew:")
	// spew.Dump(out2)

}

type Uint16Arr []uint16

func (a Uint16Arr) Len() int           { return len(a) }
func (a Uint16Arr) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a Uint16Arr) Less(i, j int) bool { return a[i] < a[j] }

//---------- pkt

type MsgH struct {
	Len uint16
	Mid uint16
	Rid uint16
}

const size_MsgH = int(unsafe.Sizeof(MsgH{}))
const size_Sid = int(unsafe.Sizeof(MsgS{}.Sid))

type MsgS struct {
	MsgH
	Sid uint32
}

func (m MsgH) String() string {
	// return fmt.Sprintf("%#v", m)
	// return fmt.Sprintf("MH{L:%+v M:%+v}", m.Len, m.Mid)
	return fmt.Sprintf("MH{n:%+v m:%+v r:%v}", m.Len, m.Mid, m.Rid)
}
func (m MsgS) String() string {
	// return fmt.Sprintf("%#v", m)
	return fmt.Sprintf("MS{n:%+v m:%+v r:%v s:%v}", m.Len, m.Mid, m.Rid, m.Sid)
}

func DecHead(buf []byte) (msgH MsgH) {
	_ = buf[size_MsgH-1]
	msgH = MsgH{
		Len: binary.BigEndian.Uint16(buf[0:2]),
		Mid: binary.BigEndian.Uint16(buf[2:4]),
		Rid: binary.BigEndian.Uint16(buf[4:6]),
	}
	return
}
func DecSess(buf []byte) (sessId uint32, ok bool) {
	if len(buf) < size_Sid {
		return 0, false
	}

	_ = buf[size_Sid-1]
	sessId = binary.BigEndian.Uint32(buf)
	ok = true
	return
}

func EncHead(msgH MsgH) (buf []byte) {
	buf = make([]byte, size_MsgH)
	binary.BigEndian.PutUint16(buf[0:2], msgH.Len)
	binary.BigEndian.PutUint16(buf[2:4], uint16(msgH.Mid))
	binary.BigEndian.PutUint16(buf[4:6], uint16(msgH.Rid))
	return
}

func EncSess(msgS MsgS) (buf []byte) {
	hdr := EncHead(msgS.MsgH)
	bdy := make([]byte, size_Sid)
	binary.BigEndian.PutUint32(bdy, msgS.Sid)
	buf = append(hdr, bdy...)

	// buf = make([]byte, 4)
	// binary.BigEndian.PutUint16(buf, msgH.Len)
	// binary.BigEndian.PutUint16(buf[2:4], uint16(msgH.Mid))
	// binary.BigEndian.PutUint32(buf[4:8], sess_id)
	return
}

//---------- pend

type PendMap map[uint16][]byte

func (p PendMap) Set(mid uint16, buf []byte) {
	// TODO: check full
	p[mid] = buf
}

func (p PendMap) Get(mid uint16) []byte {
	buf, ok := p[mid]
	if ok {
		return buf
	}
	return nil
}

func (p PendMap) Keys() []uint16 {
	keys := make([]uint16, 0, len(p))
	for k := range p {
		keys = append(keys, k)
	}
	sort.Sort(Uint16Arr(keys))
	return keys
}

// only keep (st, ed)
func (p PendMap) Trim(st, ed uint16) {
	if st <= ed {
		// (st .. k .. ed) // [st+1, ed-1]
		for k, _ := range p {
			if st < k && k <= ed {
				//keep
			} else {
				delete(p, k)
			}
		}
	} else {
		//  k..ed)  (st..k // [0,ed-1] [st+1,Max]
		for k, _ := range p {
			if st < k || k <= ed {
				//keep
			} else {
				delete(p, k)
			}
		}
	}
}

//---------- re-send
func (p PendMap) ReSend(st, ed uint16, writer net.Conn) (err int) {
	keys := Range(st, ed)

	for _, k := range keys {
		// Println(" -", k)
		if out := p.Get(k); out != nil {
			LogReMsg("- re-sendMsg:", "#", k, " len:", len(out)-4)
			_, e := writer.Write(out)
			if e != nil {
				Println("re-send err2:", err)
				return -2 //write err
			}
		} else { //err
			Println("re-send err1:", "no key", k)
			return -1 //no key
		}
	}

	return len(keys)
}

func Range(st, ed uint16) []uint16 {
	arr := make([]uint16, 0, 100)
	// Print("(", st, ed, "]")
	if st <= ed {
		for i := st + 1; i <= ed; i++ {
			// Print(" ", i)
			arr = append(arr, i)
		}
	} else {
		for i := st + 1; i > st || i <= ed; i++ {
			// Print(" ", i)
			arr = append(arr, i)
		}
	}
	// Println("")
	// Println("(", st, ed, "]", arr)

	return arr
}

//---------- sess
type Sess struct {
	Sess_id uint32
	//msg re-send
	//rid peer recv mid
	Recv_mid uint16 //peer send mid
	Self_mid uint16 //our send mid
	// pend_map map[uint16][]byte
	Pend_map PendMap
	Lock_map sync.RWMutex
}

func (s *Sess) ReadMsg(reader net.Conn) (out []byte, mid, rid int) {
	buf := make([]byte, size_MsgH)
	n, err := io.ReadFull(reader, buf)
	LogMsg("readMsgH:", err, buf)
	if err != nil || n != size_MsgH {
		Println("-- readMsgH err:", n, err)
		return nil, -1, -1
	}
	pkt := DecHead(buf)
	// Println("msgH:", pkt)

	// fmt.Fprintf(os.Stderr, "recv lock: %p\n", &s.Lock_map)
	s.Lock_map.Lock()
	LogPend("pend:", s.Pend_map.Keys())
	s.Pend_map.Trim(pkt.Rid, s.Self_mid) // <--- concurrent map iteration and map write
	LogPend("pend:", s.Pend_map.Keys())
	s.Lock_map.Unlock()

	mid = int(pkt.Mid)
	rid = int(pkt.Rid)
	out = make([]byte, pkt.Len)
	n, err = io.ReadFull(reader, out)
	LogMsg("readMsgB:", err, BufMax(out, 10))

	if err != nil {
		return nil, -1, -1
	}
	return out, mid, rid
}

func (s *Sess) SendMsg(dat []byte, writer net.Conn) error {
	hdr := EncHead(MsgH{uint16(len(dat)), s.Self_mid, s.Recv_mid})
	out := append(hdr, dat...)

	// fmt.Fprintf(os.Stderr, "send lock: %p\n", &s.Lock_map)
	s.Lock_map.Lock()
	s.Pend_map.Set(s.Self_mid, out) // <--- concurrent map iteration and map write
	s.Lock_map.Unlock()

	LogMsg("- sendMsg:", len(dat), hdr, BufMax(dat, 10))
	_, err := writer.Write(out)
	if err != nil {
		return err
	}
	return nil
}

func (s *Sess) SendSession(writer net.Conn) {
	hdr := MsgH{Len: uint16(size_Sid), Mid: s.Self_mid, Rid: s.Recv_mid} //SessMsg not need mid
	out := EncSess(MsgS{hdr, s.Sess_id})

	LogSessMsg("- sendSess:", "#", s.Self_mid, s.Recv_mid, out)
	_, err := writer.Write(out)
	if err != nil {
		Println("send-sess err1:", err)
		return
	}
}

// type SessionC struct {
// 	Session
// }
