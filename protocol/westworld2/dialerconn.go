package westworld2

import (
	"github.com/michaelquigley/dilithium/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"net"
	"time"
)

type dialerConn struct {
	conn     *net.UDPConn
	peer     *net.UDPAddr
	seq      *util.Sequence
	txPortal3 *txPortal3
	rxPortal2 *rxPortal2
	pool     *pool
	ins      Instrument
}

func newDialerConn(conn *net.UDPConn, peer *net.UDPAddr, ins Instrument) *dialerConn {
	dc := &dialerConn{
		conn: conn,
		peer: peer,
		seq:  util.NewSequence(0),
		pool: newPool("dialerConn", ins),
		ins:  ins,
	}
	dc.txPortal3 = newTxPortal3(conn, peer, ins)
	dc.rxPortal2 = newRxPortal2(conn, peer, ins)
	return dc
}

func (self *dialerConn) Read(p []byte) (int, error) {
	return self.rxPortal2.read(p)
}

func (self *dialerConn) Write(p []byte) (int, error) {
	if err := self.txPortal3.tx(newData(self.seq.Next(), p, self.pool)); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (self *dialerConn) Close() error {
	return nil
}

func (self *dialerConn) RemoteAddr() net.Addr {
	return self.peer
}

func (self *dialerConn) LocalAddr() net.Addr {
	return self.conn.LocalAddr()
}

func (self *dialerConn) SetDeadline(t time.Time) error {
	return nil
}

func (self *dialerConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (self *dialerConn) SetWriteDeadline(t time.Time) error {
	return nil
}

func (self *dialerConn) rxer() {
	logrus.Info("started")
	defer logrus.Warn("exited")

	for {
		wm, _, err := readWireMessage(self.conn, self.pool, self.ins)
		if err != nil {
			if self.ins != nil {
				self.ins.readError(self.peer, err)
			}
			continue
		}

		if wm.mt == DATA {
			if wm.ack != -1 {
				self.txPortal3.ack(wm.ack)
			}
			self.rxPortal2.rx(wm)

		} else if wm.mt == ACK {
			if wm.ack != -1 {
				self.txPortal3.ack(wm.ack)
			}
			wm.buffer.unref()

		} else {
			if self.ins != nil {
				self.ins.unexpectedMessageType(self.peer, wm.mt)
			}
			wm.buffer.unref()
		}
	}
}

func (self *dialerConn) hello() error {
	/*
	 * Send Hello
	 */
	helloSeq := self.seq.Next()
	hello := newHello(helloSeq, self.pool)
	defer hello.buffer.unref()

	if err := writeWireMessage(hello, self.conn, self.peer, self.ins); err != nil {
		return errors.Wrap(err, "write hello")
	}
	/* */

	/*
	 * Expect Ack'd Hello Response
	 */
	if err := self.conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return errors.Wrap(err, "set read deadline")
	}

	helloAck, _, err := readWireMessage(self.conn, self.pool, self.ins)
	if err != nil {
		return errors.Wrap(err, "read hello ack")
	}
	defer helloAck.buffer.unref()

	if helloAck.mt != HELLO {
		return errors.Wrap(err, "unexpected response")
	}
	if helloAck.ack != helloSeq {
		return errors.New("invalid hello ack")
	}
	if err := self.conn.SetReadDeadline(time.Time{}); err != nil {
		return errors.Wrap(err, "clear read deadline")
	}
	/* */

	// The next sequence should be the next highest sequence
	self.rxPortal2.setAccepted(helloAck.seq)

	/*
	 * Send Final Ack
	 */
	ack := newAck(helloAck.seq, self.pool)
	defer ack.buffer.unref()

	if err := writeWireMessage(ack, self.conn, self.peer, self.ins); err != nil {
		return errors.Wrap(err, "write ack")
	}
	/* */

	if self.ins != nil {
		self.ins.connected(self.peer)
	}

	go self.rxer()

	return nil
}
