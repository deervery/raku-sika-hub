package qlbackend

import "fmt"

// Media is the roll a job was encoded for, from its print information
// command (ESC i z).
type Media struct {
	Known   bool
	Type    byte
	WidthMM byte
}

// Job is what the backend needs to know about a raster stream before sending
// it: how many labels to expect a completion for, and which roll it needs.
type Job struct {
	Pages int
	Media Media
	// TwoColor: the job is already in two-colour mode (ESC i K bit 0, or
	// two-colour raster lines).
	TwoColor bool
}

// ParseJob walks the QL raster command stream command by command. Walking it,
// rather than searching for byte patterns, keeps raster rows — which can hold
// any byte values — from being mistaken for commands.
//
// A stream it cannot follow is an error: sending bytes the backend does not
// understand would leave it unable to tell when the printer has finished.
func ParseJob(data []byte) (Job, error) {
	var job Job
	i := 0
	need := func(n int) error {
		if i+n > len(data) {
			return fmt.Errorf("qlbackend: stream ends inside a command at byte %d", i)
		}
		return nil
	}
	for i < len(data) {
		switch c := data[i]; c {
		case 0x00: // invalidate
			i++
		case 0x0C, 0x1A: // print (0x1A: print with feed, last page)
			job.Pages++
			i++
		case 'M': // compression mode
			if err := need(2); err != nil {
				return Job{}, err
			}
			i += 2
		case 'Z': // zero raster line
			i++
		case 'g': // raster line: g 0x00 n + n bytes
			if err := need(3); err != nil {
				return Job{}, err
			}
			n := int(data[i+2])
			if err := need(3 + n); err != nil {
				return Job{}, err
			}
			i += 3 + n
		case 'w': // two-colour raster line: w colour n + n bytes
			if err := need(3); err != nil {
				return Job{}, err
			}
			n := int(data[i+2])
			if err := need(3 + n); err != nil {
				return Job{}, err
			}
			job.TwoColor = true
			i += 3 + n
		case 'G': // compressed raster line: G n1 n2 + (n1 | n2<<8) bytes
			if err := need(3); err != nil {
				return Job{}, err
			}
			n := int(data[i+1]) | int(data[i+2])<<8
			if err := need(3 + n); err != nil {
				return Job{}, err
			}
			i += 3 + n
		case 0x1B:
			if err := need(2); err != nil {
				return Job{}, err
			}
			if data[i+1] == '@' { // initialize
				i += 2
				continue
			}
			if data[i+1] != 'i' {
				return Job{}, fmt.Errorf("qlbackend: unknown command ESC 0x%02x at byte %d", data[i+1], i)
			}
			if err := need(3); err != nil {
				return Job{}, err
			}
			var size int
			switch data[i+2] {
			case 'S': // status request
				size = 3
			case 'a', 'M', 'A', 'K': // mode switch, various mode, cut every, expanded mode
				size = 4
			case 'd': // margin
				size = 5
			case 'z': // print information
				size = 13
			default:
				return Job{}, fmt.Errorf("qlbackend: unknown command ESC i 0x%02x at byte %d", data[i+2], i)
			}
			if err := need(size); err != nil {
				return Job{}, err
			}
			if data[i+2] == 'K' && data[i+3]&expandedTwoColor != 0 {
				job.TwoColor = true
			}
			if data[i+2] == 'z' && !job.Media.Known {
				flags := data[i+3]
				// Only trust the fields the job marks as valid.
				if flags&0x02 != 0 && flags&0x04 != 0 {
					job.Media = Media{Known: true, Type: data[i+4], WidthMM: data[i+5]}
				}
			}
			i += size
		default:
			return Job{}, fmt.Errorf("qlbackend: unknown byte 0x%02x at byte %d", c, i)
		}
	}
	if job.Pages == 0 {
		return Job{}, fmt.Errorf("qlbackend: stream has no print command")
	}
	return job, nil
}

// expandedTwoColor is the two-colour printing bit of ESC i K.
const expandedTwoColor = 0x01

// ToTwoColor rewrites a black-only job for a red/black roll: two-colour
// printing is switched on (ESC i K) and every raster line is sent as a black
// line with an empty red line after it (w 0x01 / w 0x02, as brother_ql sends
// for 62red). Nothing else changes. Only the uncompressed lines qlraster
// writes are converted; anything else is an error, and the job is then sent
// as it was.
func ToTwoColor(data []byte) ([]byte, error) {
	out := make([]byte, 0, len(data)*2)
	for i := 0; i < len(data); {
		switch c := data[i]; c {
		case 'g':
			if i+3 > len(data) || data[i+1] != 0x00 {
				return nil, fmt.Errorf("qlbackend: raster line at byte %d cannot be converted", i)
			}
			n := int(data[i+2])
			if i+3+n > len(data) {
				return nil, fmt.Errorf("qlbackend: stream ends inside a raster line at byte %d", i)
			}
			out = append(out, 'w', 0x01, byte(n))
			out = append(out, data[i+3:i+3+n]...)
			out = append(out, 'w', 0x02, byte(n))
			out = append(out, make([]byte, n)...)
			i += 3 + n
		case 'G', 'Z', 'M', 'w':
			return nil, fmt.Errorf("qlbackend: command 0x%02x at byte %d cannot be converted", c, i)
		case 0x1B:
			if i+2 > len(data) {
				return nil, fmt.Errorf("qlbackend: stream ends inside a command at byte %d", i)
			}
			if data[i+1] == '@' {
				out = append(out, data[i:i+2]...)
				i += 2
				continue
			}
			if i+3 > len(data) || data[i+1] != 'i' {
				return nil, fmt.Errorf("qlbackend: unknown command at byte %d", i)
			}
			size := 0
			switch data[i+2] {
			case 'S':
				size = 3
			case 'a', 'M', 'A', 'K':
				size = 4
			case 'd':
				size = 5
			case 'z':
				size = 13
			default:
				return nil, fmt.Errorf("qlbackend: unknown command ESC i 0x%02x at byte %d", data[i+2], i)
			}
			if i+size > len(data) {
				return nil, fmt.Errorf("qlbackend: stream ends inside a command at byte %d", i)
			}
			start := len(out)
			out = append(out, data[i:i+size]...)
			if data[i+2] == 'K' {
				out[start+3] |= expandedTwoColor
			}
			i += size
		default: // invalidate (0x00), print (0x0C, 0x1A)
			out = append(out, c)
			i++
		}
	}
	return out, nil
}
