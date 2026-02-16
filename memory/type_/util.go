package type_

type idRing struct {
	buf   []string
	head  int
	size  int
	index map[string]int
}

func newIDRing(capHint int) *idRing {
	initial := capHint
	if initial < 16 {
		initial = 16
	}
	return &idRing{
		buf:   make([]string, initial),
		index: make(map[string]int, initial),
	}
}

func (r *idRing) Len() int {
	if r == nil {
		return 0
	}
	return r.size
}

func (r *idRing) Contains(id string) bool {
	_, ok := r.index[id]
	return ok
}

func (r *idRing) PushBack(id string) {
	if id == "" || r.Contains(id) {
		return
	}
	r.ensureCapacity(r.size + 1)
	pos := (r.head + r.size) % len(r.buf)
	r.buf[pos] = id
	r.index[id] = pos
	r.size++
}

func (r *idRing) PopFront() (string, bool) {
	if r.size == 0 {
		return "", false
	}
	pos := r.head
	id := r.buf[pos]
	delete(r.index, id)
	r.buf[pos] = ""
	r.head = (r.head + 1) % len(r.buf)
	r.size--
	if r.size == 0 {
		r.head = 0
	}
	return id, true
}

func (r *idRing) Remove(id string) bool {
	if !r.Contains(id) {
		return false
	}
	if r.size == 0 {
		return false
	}
	vals := r.Values()
	r.Reset()
	for _, current := range vals {
		if current == id {
			continue
		}
		r.PushBack(current)
	}
	return true
}

func (r *idRing) Values() []string {
	if r.size == 0 {
		return nil
	}
	out := make([]string, 0, r.size)
	for i := 0; i < r.size; i++ {
		pos := (r.head + i) % len(r.buf)
		if r.buf[pos] != "" {
			out = append(out, r.buf[pos])
		}
	}
	return out
}

func (r *idRing) Reset() {
	if r == nil {
		return
	}
	for i := range r.buf {
		r.buf[i] = ""
	}
	r.head = 0
	r.size = 0
	r.index = make(map[string]int, len(r.buf))
}

func (r *idRing) ensureCapacity(need int) {
	if len(r.buf) >= need {
		return
	}
	newCap := len(r.buf) * 2
	if newCap < need {
		newCap = need
	}
	if newCap < 16 {
		newCap = 16
	}
	newBuf := make([]string, newCap)
	newIndex := make(map[string]int, newCap)
	for i := 0; i < r.size; i++ {
		oldPos := (r.head + i) % len(r.buf)
		id := r.buf[oldPos]
		newBuf[i] = id
		if id != "" {
			newIndex[id] = i
		}
	}
	r.buf = newBuf
	r.index = newIndex
	r.head = 0
}
