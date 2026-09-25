package qlbackend

import (
	"errors"
	"sync"
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
	muted        bool // answers nothing, as after a job through CUPS's usb backend
	busyReplies  int  // replies to report as busy before a normal one
	failOnPrint  *[2]byte
	noCompletion bool
	dropOnPrint  bool // disconnect when the first label is printed

	statusRequests int
	printed        int
}

func newFakePrinter() *fakePrinter {
	p := &fakePrinter{width: 62, media: 0x0A}
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
		case len(cmd) == 3 && cmd[0] == 0x1B && cmd[1] == 'i' && cmd[2] == 'S':
			p.statusRequests++
			if p.muted {
				continue
			}
			e1 := p.err1
			if p.busyReplies > 0 {
				p.busyReplies--
				e1 |= 0x10
			}
			p.out = append(p.out, p.frame(TypeReply, 0x00, e1, p.err2)...)
		case cmd[0] == 0x0C || cmd[0] == 0x1A:
			p.printed++
			if p.muted {
				continue
			}
			if p.dropOnPrint {
				p.closed = true
				continue
			}
			if p.failOnPrint != nil {
				p.out = append(p.out, p.frame(TypeErrorOccurred, 0x00, p.failOnPrint[0], p.failOnPrint[1])...)
				continue
			}
			p.out = append(p.out, p.frame(TypePhaseChange, 0x01, 0, 0)...)
			if !p.noCompletion {
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
	case 'g':
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
