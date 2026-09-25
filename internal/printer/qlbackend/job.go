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
