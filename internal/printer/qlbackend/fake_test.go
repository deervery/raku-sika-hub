package qlbackend

import (
	"errors"
	"sync"
	"time"
)

// fakePrinter behaves like the QL-820NWB on office: it answers ESC i S with a
// status frame and reports each label with phase change → printing completed
// → phase change. Fields switch on the failures the backend must report.
type fakePrinter struct {
	mu     sync.Mutex
	cond   *sync.Cond
	out    []byte // frames waiting to be read
	in     []byte // bytes written, not yet interpreted
	all    []byte // every byte written
	closed bool

	err1, err2   byte
	width, media byte
	textColor    byte // status byte 25; 0x81 for a red/black roll
	muted        bool // answers nothing, as after a job through CUPS's usb backend
	busyReplies  int  // replies to report as busy before a normal one
	failOnPrint  *[2]byte
	noCompletion bool
	// After a label: stop answering anything (silentAfterPrint), set error
	// bits without saying so (errAfterPrint), or report the completion only
	// once asked for the status (completeOnAsk).
	silentAfterPrint bool
	completeOnly     int // report only this many labels, then go quiet (still answering)
	muteAfterError   bool // answer nothing once failOnPrint has been reported
	errAfterPrint    *[2]byte
	completeOnAsk    bool
	askCompleted     bool
	dropOnPrint  bool // disconnect when the first label is printed
	coolFor      time.Duration // pause to cool the head before completing

	statusRequests int
	printed        int

	twoColorMode  bool // ESC i K bit 0 seen
	twoColorLines int  // w raster lines seen
	blackLines    int  // w 0x01 lines with any ink
	redLines      int  // w 0x02 lines with any ink
}

func newFakePrinter() *fakePrinter {
	p := &fakePrinter{width: 62, media: 0x0A, textColor: 0x01}
	p.cond = sync.NewCond(&p.mu)
	return p
}

func (p *fakePrinter) frame(typ, phase, err1, err2 byte) []byte {
	b := make([]byte, StatusSize)
	b[0], b[1], b[2] = 0x80, 0x20, 0x42
	b[3], b[4] = 0x34, 0x41
	b[8], b[9] = err1, err2
	b[10], b[11] = p.width, p.media
	b[18], b[19] = typ, phase
	b[25] = p.textColor
	return b
}

func (p *fakePrinter) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return 0, errors.New("closed")
	}
	p.in = append(p.in, b...)
	p.all = append(p.all, b...)
	p.interpret()
	p.cond.Broadcast()
	return len(b), nil
}

// interpret consumes every complete command in p.in.
func (p *fakePrinter) interpret() {
	for len(p.in) > 0 {
		n := commandSize(p.in)
		if n == 0 || n > len(p.in) {
			return
		}
		cmd := p.in[:n]
		p.in = p.in[n:]
		switch {
		case len(cmd) == 4 && cmd[0] == 0x1B && cmd[1] == 'i' && cmd[2] == 'K':
			p.twoColorMode = cmd[3]&0x01 != 0
		case cmd[0] == 'w':
			p.twoColorLines++
			ink := false
			for _, c := range cmd[3:] {
				ink = ink || c != 0
			}
			if ink && cmd[1] == 0x01 {
				p.blackLines++
			}
			if ink && cmd[1] == 0x02 {
				p.redLines++
			}
		case len(cmd) == 3 && cmd[0] == 0x1B && cmd[1] == 'i' && cmd[2] == 'S':
			p.statusRequests++
			if p.muted || (p.silentAfterPrint && p.printed > 0) {
				continue
			}
			if p.completeOnAsk && p.printed > 0 && !p.askCompleted {
				p.askCompleted = true
				p.out = append(p.out, p.frame(TypePrintingCompleted, 0x01, 0, 0)...)
			}
			e1 := p.err1
			if p.busyReplies > 0 {
				p.busyReplies--
				e1 |= 0x10
			}
			// Like the QL-820NWB: with an error pending, the answer to a
			// status request is typed "error occurred", not "reply".
			typ := TypeReply
			if p.err1&^0x10 != 0 || p.err2 != 0 {
				typ = TypeErrorOccurred
			}
			p.out = append(p.out, p.frame(typ, 0x00, e1, p.err2)...)
		case cmd[0] == 0x0C || cmd[0] == 0x1A:
			p.printed++
			if p.muted || p.silentAfterPrint {
				continue
			}
			if p.errAfterPrint != nil {
				p.err1, p.err2 = p.errAfterPrint[0], p.errAfterPrint[1]
				continue
			}
			if p.dropOnPrint {
				p.closed = true
				continue
			}
			if p.failOnPrint != nil {
				p.out = append(p.out, p.frame(TypeErrorOccurred, 0x00, p.failOnPrint[0], p.failOnPrint[1])...)
				p.muted = p.muteAfterError
				continue
			}
			// Like hakodate's QL-820NWB: a black-only job on a red/black
			// roll starts, then stops with no error bits set.
			if p.textColor&0x80 != 0 && !p.twoColorMode {
				p.out = append(p.out, p.frame(TypePhaseChange, 0x01, 0, 0)...)
				p.out = append(p.out, p.frame(TypeErrorOccurred, 0x01, 0, 0)...)
				continue
			}
			p.out = append(p.out, p.frame(TypePhaseChange, 0x01, 0, 0)...)
			if p.coolFor > 0 {
				cool := p.frame(TypeNotification, 0x01, 0, 0)
				cool[22] = NotifyCoolingStarted
				p.out = append(p.out, cool...)
				go p.finishAfterCooling()
				continue
			}
			if !p.noCompletion && (p.completeOnly == 0 || p.printed <= p.completeOnly) {
				p.out = append(p.out, p.frame(TypePrintingCompleted, 0x01, 0, 0)...)
			}
			p.out = append(p.out, p.frame(TypePhaseChange, 0x00, 0, 0)...)
		}
	}
}

// commandSize is the length of the command at the start of b, 0 if unknown.
func commandSize(b []byte) int {
	switch b[0] {
	case 0x00, 0x0C, 0x1A, 'Z':
		return 1
	case 'M':
		return 2
	case 'g', 'w':
		if len(b) < 3 {
			return 3
		}
		return 3 + int(b[2])
	case 0x1B:
		if len(b) < 3 {
			return 3
		}
		if b[1] == '@' {
			return 2
		}
		switch b[2] {
		case 'S':
			return 3
		case 'a', 'M', 'A', 'K':
			return 4
		case 'd':
			return 5
		case 'z':
			return 13
		}
	}
	return 0
}

func (p *fakePrinter) Read(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for len(p.out) == 0 && !p.closed {
		p.cond.Wait()
	}
	if len(p.out) == 0 {
		return 0, errors.New("closed")
	}
	n := copy(b, p.out)
	p.out = p.out[n:]
	return n, nil
}

func (p *fakePrinter) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
	p.cond.Broadcast()
	return nil
}

func (p *fakePrinter) written() []byte {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]byte(nil), p.all...)
}

func (p *fakePrinter) requests() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.statusRequests
}

func (p *fakePrinter) finishAfterCooling() {
	time.Sleep(p.coolFor)
	p.mu.Lock()
	defer p.mu.Unlock()
	done := p.frame(TypeNotification, 0x01, 0, 0)
	done[22] = NotifyCoolingFinished
	p.out = append(p.out, done...)
	p.out = append(p.out, p.frame(TypePrintingCompleted, 0x01, 0, 0)...)
	p.out = append(p.out, p.frame(TypePhaseChange, 0x00, 0, 0)...)
	p.cond.Broadcast()
}

// queue puts frames in front of anything the printer says next, as if an
// earlier job had left them unread.
func (p *fakePrinter) queue(frames ...[]byte) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, f := range frames {
		p.out = append(p.out, f...)
	}
	p.cond.Broadcast()
}
