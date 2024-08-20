package packetreader

import "io"

type Reader struct {
	buf []byte
	pos int
}

func NewReader(b []byte) *Reader {
	return &Reader{
		buf: b,
		pos: 0,
	}
}

func (r *Reader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.buf) {
		return 0, io.EOF
	}
	n = copy(p, r.buf[r.pos:])
	r.pos += n
	return n, nil
}
