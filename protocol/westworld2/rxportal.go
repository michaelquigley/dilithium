package westworld2

import (
	"bytes"
	"github.com/emirpasic/gods/trees/btree"
	"github.com/emirpasic/gods/utils"
	"github.com/michaelquigley/dilithium/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"io"
	"math"
	"net"
	"sync"
)

type rxPortal struct {
	tree       *btree.Tree
	accepted   int32
	closed     bool
	rxs        chan *wireMessage
	reads      chan *rxRead
	readBuffer *bytes.Buffer
	rxPortalSz int32
	readPool   *sync.Pool
	ackPool    *pool
	conn       *net.UDPConn
	peer       *net.UDPAddr
	txPortal   *txPortal
	seq        *util.Sequence
	config     *Config
	count      int
}

type rxRead struct {
	buf []byte
	sz  int
	eof bool
}

func newRxPortal(conn *net.UDPConn, peer *net.UDPAddr, txPortal *txPortal, seq *util.Sequence, config *Config) *rxPortal {
	rxp := &rxPortal{
		tree:       btree.NewWith(config.treeLen, utils.Int32Comparator),
		accepted:   -1,
		rxs:        make(chan *wireMessage),
		reads:      make(chan *rxRead, config.readsQLen),
		readBuffer: new(bytes.Buffer),
		readPool:   new(sync.Pool),
		ackPool:    newPool("ackPool", config),
		conn:       conn,
		peer:       peer,
		txPortal:   txPortal,
		seq:        seq,
		config:     config,
		count:      0,
	}
	rxp.readPool.New = func() interface{} {
		return make([]byte, config.poolBufferSz)
	}
	go rxp.run()
	return rxp
}

func (self *rxPortal) read(p []byte) (int, error) {
preread:
	for {
		select {
		case read, ok := <-self.reads:
			if !ok {
				return 0, io.EOF
			}
			if !read.eof {
				n, err := self.readBuffer.Write(read.buf[:read.sz])
				if err != nil {
					return 0, errors.Wrap(err, "buffer")
				}
				if n != read.sz {
					return 0, errors.Wrap(err, "short buffer")
				}
			}

		default:
			break preread
		}
	}
	if self.readBuffer.Len() > 0 {
		return self.readBuffer.Read(p)
	} else {
		read, ok := <-self.reads
		if !ok {
			return 0, io.EOF
		}
		if !read.eof {
			n, err := self.readBuffer.Write(read.buf[:read.sz])
			if err != nil {
				return 0, errors.Wrap(err, "buffer")
			}
			if n != read.sz {
				return 0, errors.Wrap(err, "short buffer")
			}
			self.readPool.Put(read.buf)

			return self.readBuffer.Read(p)

		} else {
			logrus.Infof("close notified")
			close(self.reads)
			return 0, io.EOF
		}
	}
}

func (self *rxPortal) rx(wm *wireMessage) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.Wrap(err, "send on closed rxs")
		}
	}()
	self.rxs <- wm
	return err
}

func (self *rxPortal) setAccepted(accepted int32) {
	self.accepted = accepted
}

func (self *rxPortal) run() {
	logrus.Info("started")
	defer logrus.Warn("exited")

	defer func() {
		if r := recover(); r != nil {
			logrus.Errorf("recovered (%v)", r)
		}
	}()

	for {
		wm, ok := <-self.rxs
		if !ok {
			return
		}

		self.count++
		if self.count%treeReportCt == 0 {
			logrus.Infof("tree.Size = %d, rxPortalSz = %d", self.tree.Size(), self.rxPortalSz)
		}

		if wm.seq > self.accepted || (self.accepted == math.MaxInt32 && wm.seq == 0) {
			self.tree.Put(wm.seq, wm)
			self.rxPortalSz += int32(len(wm.data))
		} else {
			if self.config.i != nil {
				self.config.i.duplicateRx(self.peer, wm)
			}
			wm.buffer.unref()
		}

		ack := newAck(wm.seq, self.rxPortalSz, self.ackPool)
		if wm.mf&RTT == 1 {
			ts, err := wm.readRtt()
			if err == nil {
				ack.writeRtt(ts)
			} else {
				logrus.Errorf("error reading rtt (%v)", err)
			}
		}
		if err := writeWireMessage(ack, self.conn, self.peer); err != nil {
			logrus.Errorf("error sending ack (%v)", err)
		}
		if self.config.i != nil {
			self.config.i.wireMessageTx(self.peer, ack)
		}
		ack.buffer.unref()

		if self.tree.Size() > 0 {
			var next int32
			if self.accepted < math.MaxInt32 {
				next = self.accepted + 1
			} else {
				next = 0
			}

			for _, key := range self.tree.Keys() {
				if key.(int32) == next {
					v, _ := self.tree.Get(key)
					wm := v.(*wireMessage)
					buf := self.readPool.Get().([]byte)
					n := copy(buf, wm.data)
					self.reads <- &rxRead{buf, n, false}

					self.tree.Remove(key)
					self.rxPortalSz -= int32(len(wm.data))
					wm.buffer.unref()
					self.accepted = next
					if next < math.MaxInt32 {
						next++
					} else {
						next = 0
					}
				}
			}
		}

		if wm.mt == CLOSE && !self.closed {
			self.txPortal.close(self.seq)
			self.reads <- &rxRead{nil, 0, true}
			self.closed = true
			close(self.rxs)
		}
	}
}
