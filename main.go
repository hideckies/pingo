package main

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"sync"
	"time"

	"github.com/hideckies/pingo/sub"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

const (
	PROTO_NUM_ICMP_IPv4 = 1
	PROTO_NUM_ICMP_IPv6 = 58
)

type Pingo struct {
	Count    int
	Host     string
	Interval time.Duration
	Packet   sub.Packet

	// Channel
	done chan interface{}
	mtx  sync.Mutex
}

// Validate interval
func (p *Pingo) ValidInterval(interval string) bool {
	r, _ := regexp.Compile(`^([1-9][0-9]*|0)`)
	return r.MatchString(interval)
}

// Run
// Reference: https://pkg.go.dev/golang.org/x/net@v0.0.0-20221004154528-8021a29435af/icmp#example-PacketConn-NonPrivilegedPing
func (p *Pingo) Run() error {
	lis, err := icmp.ListenPacket(p.Packet.Proto+":icmp", p.Packet.SrcAddr.String())
	if err != nil {
		log.Fatalf("ICMP ListenPacket Error: %v\n", err)
	}
	defer lis.Close()

	// var msgType icmp.Type
	// if p.Packet.Proto == "ipv4" {
	// 	msgType = ipv4.ICMPTypeEcho
	// } else if p.Packet.Proto == "ipv6" {
	// 	msgType = ipv6.ICMPTypeEchoRequest
	// }

	c := 1
	for range time.Tick(p.Interval) {
		wm := icmp.Message{
			Type: ipv4.ICMPTypeEcho,
			Code: 0,
			Body: &icmp.Echo{
				ID:   os.Getpid() & 0xffff,
				Seq:  c,
				Data: []byte("PINGO"),
			},
		}
		wb, err := wm.Marshal(nil)
		if err != nil {
			log.Fatalf("Marshal Error: %v\n", err)
		}
		if _, err := lis.WriteTo(wb, p.Packet.DestAddr); err != nil {
			log.Fatalf("WriteTo Error: %v\n", err)
		}

		rb := make([]byte, 1500)
		n, peer, err := lis.ReadFrom(rb)
		if err != nil {
			log.Fatalf("ReadFrom Error: %v\n", err)
		}

		protoNum := PROTO_NUM_ICMP_IPv4
		if p.Packet.Proto == "ip6" {
			protoNum = PROTO_NUM_ICMP_IPv6
		}
		rm, err := icmp.ParseMessage(protoNum, rb[:n])
		if err != nil {
			log.Fatal(err)
		}
		switch rm.Type {
		case ipv4.ICMPTypeEchoReply:
			log.Printf("got reflection from %v\n", peer)
		default:
			log.Printf("got %+v; want echo reply\n", rm)
		}

		c++

		if p.Count != 0 && c > p.Count {
			break
		}
	}

	return nil
}

func (p *Pingo) Stop() {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	open := true
	select {
	case _, open = <-p.done:
	default:
	}

	if open {
		close(p.done)
	}
}

func NewPingo(flag sub.Flag, packet sub.Packet) *Pingo {
	var p Pingo
	p.Count = flag.Count
	p.Host = flag.Target
	p.Packet = packet
	p.done = make(chan interface{})

	if !p.ValidInterval(flag.Interval) {
		fmt.Println(sub.ERROR_INCORRECT_VALUE_INTERVAL)
		flag.Interval = "1"
	}
	interval, err := time.ParseDuration(flag.Interval + "s")
	if err == nil {
		p.Interval = interval
	} else {
		fmt.Println(sub.ERROR_INCORRECT_VALUE_INTERVAL)
		p.Interval = 1 * time.Second
	}

	return &p
}

func main() {
	var f sub.Flag

	err := f.Parse()
	if err != nil {
		fmt.Println(err)
		return
	}

	packet := sub.NewPacket(f)
	pingo := NewPingo(f, *packet)

	fmt.Printf("pingo %s (%s)\n", pingo.Host, pingo.Packet.DestAddr.String())

	// Listen for Ctrl+c signal
	// cch := make(chan os.Signal, 1)
	// signal.Notify(cch, os.Interrupt)
	// go func() {
	// 	for range cch {
	// 		pingo.Stop()
	// 	}
	// }()

	err = pingo.Run()
	if err != nil {
		fmt.Println("Error pingo")
	}
}
