package anonbcast

// RespChannels provides a simple interface for storing channels over which messages can be sent.
// It is NOT thread safe (but the individual channels are, of course).
type RespChannels interface {
	Create(i int, j int) chan bool
	Get(i int, j int) chan bool
	Close(i int, j int)
	Clear()
}

type respChannelsImpl struct {
	m map[int]map[int]chan bool
}

func NewRespChannels() RespChannels {
	return &respChannelsImpl{m: make(map[int]map[int]chan bool)}
}

func (r *respChannelsImpl) Create(i int, j int) chan bool {
	assertf(r.Get(i, j) == nil, "cannot create existing channel!")
	ch := make(chan bool)
	if r.m[i] == nil {
		r.m[i] = make(map[int]chan bool)
	}
	r.m[i][j] = ch
	return ch
}

func (r *respChannelsImpl) Get(i int, j int) chan bool {
	mm := r.m[i]
	if mm == nil {
		return nil
	}
	return mm[j]
}

func (r *respChannelsImpl) Close(i int, j int) {
	assertf(r.Get(i, j) != nil, "cannot close nonexisting channel!")
	close(r.Get(i, j))
	delete(r.m[i], j)
	if len(r.m[i]) == 0 {
		delete(r.m, i)
	}
}

func (r *respChannelsImpl) Clear() {
	for term := range r.m {
		for index := range r.m[term] {
			close(r.m[term][index])
		}
	}
	r.m = make(map[int]map[int]chan bool)
}
