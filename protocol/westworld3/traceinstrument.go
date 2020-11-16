package westworld3

import (
	"fmt"
	"net"
	"strings"
	"sync"
)

type traceInstrument struct{}

type traceInstrumentInstance struct {
	id   string
	peer *net.UDPAddr
	lock *sync.Mutex
}

func NewTraceInstrument() Instrument {
	return &traceInstrument{}
}

func (self *traceInstrument) NewInstance(id string, peer *net.UDPAddr) InstrumentInstance {
	return &traceInstrumentInstance{id, peer, new(sync.Mutex)}
}

/*
 * connection
 */
func (self *traceInstrumentInstance) Listener(addr *net.UDPAddr) {
}

func (self *traceInstrumentInstance) Hello(peer *net.UDPAddr) {
}

func (self *traceInstrumentInstance) Connected(peer *net.UDPAddr) {
}

func (self *traceInstrumentInstance) ConnectionError(peer *net.UDPAddr, err error) {
}

func (self *traceInstrumentInstance) Closed(peer *net.UDPAddr) {
}

/*
 * wire
 */
func (self *traceInstrumentInstance) WireMessageTx(peer *net.UDPAddr, wm *wireMessage) {
	decode, _ := self.decode(wm)
	self.lock.Lock()
	fmt.Println(fmt.Sprintf("&& %-24s %-8s #%-8d %s {%s} -> %s", self.id, "TX", wm.seq, wm.messageType(), wm.mt.FlagsString(), decode))
	self.lock.Unlock()
}

func (self *traceInstrumentInstance) WireMessageRetx(peer *net.UDPAddr, wm *wireMessage) {
	decode, _ := self.decode(wm)
	self.lock.Lock()
	fmt.Println(fmt.Sprintf("&& %-24s %-8s #%-8d %s {%s} -> %s", self.id, "RETX", wm.seq, wm.messageType(), wm.mt.FlagsString(), decode))
	self.lock.Unlock()
}

func (self *traceInstrumentInstance) WireMessageRx(peer *net.UDPAddr, wm *wireMessage) {
	decode, _ := self.decode(wm)
	self.lock.Lock()
	fmt.Println(fmt.Sprintf("&& %-24s %-8s #%-8d %s {%s} -> %s", self.id, "RX", wm.seq, wm.messageType(), wm.mt.FlagsString(), decode))
	self.lock.Unlock()
}

func (self *traceInstrumentInstance) UnknownPeer(peer *net.UDPAddr) {
}

func (self *traceInstrumentInstance) ReadError(peer *net.UDPAddr, err error) {
}

func (self *traceInstrumentInstance) UnexpectedMessageType(peer *net.UDPAddr, mt messageType) {
}

/*
 * txPortal
 */
func (self *traceInstrumentInstance) TxPortalCapacityChanged(peer *net.UDPAddr, capacity int) {
}

func (self *traceInstrumentInstance) TxPortalSzChanged(peer *net.UDPAddr, sz int) {
}

func (self *traceInstrumentInstance) TxPortalRxSzChanged(peer *net.UDPAddr, sz int) {
}

func (self *traceInstrumentInstance) NewRetxMs(peer *net.UDPAddr, retxMs int) {
}

func (self *traceInstrumentInstance) DuplicateAck(peer *net.UDPAddr, seq int32) {
}

/*
 * rxPortal
 */
func (self *traceInstrumentInstance) RxPortalSzChanged(peer *net.UDPAddr, sz int) {
}

func (self *traceInstrumentInstance) DuplicateRx(peer *net.UDPAddr, wm *wireMessage) {
}

/*
 * allocation
 */
func (self *traceInstrumentInstance) Allocate(id string) {
}

/*
 * instrument lifecycle
 */
func (self *traceInstrumentInstance) Shutdown() {
}

func (self *traceInstrumentInstance) decode(wm *wireMessage) (string, error) {
	out := ""
	switch wm.messageType() {
	case HELLO:
		h, acks, err := wm.asHello()
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("{v:%d, p:%d} |%s|", h.version, h.profile, self.decodeAcks(acks)), nil

	case ACK:
		a, rxPortalSz, _, err := wm.asAck()
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("|%s| %%%d", self.decodeAcks(a), rxPortalSz), nil

	case DATA:
		sz, err := wm.asDataSize()
		if err != nil {
			return "", err
		}
		return fmt.Sprintf(":%d", sz), nil

	default:
		return out, nil
	}
}

func (self *traceInstrumentInstance) decodeAcks(acks []ack) string {
	out := ""
	for _, ack := range acks {
		if ack.start == ack.end {
			out += fmt.Sprintf(" @%d", ack.start)
		} else {
			out += fmt.Sprintf(" @%d:%d", ack.start, ack.end)
		}
	}
	return strings.TrimSpace(out)
}
