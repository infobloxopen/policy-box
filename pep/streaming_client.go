package pep

import (
	"fmt"
	"sync/atomic"

	"github.com/allegro/bigcache/v2"
	pb "github.com/infobloxopen/themis/pdp-service"
)

const (
	scsDisconnected uint32 = iota
	scsConnecting
	scsConnected
	scsClosing
	scsClosed
)

type validator func(m *pb.Msg) (pb.Msg, error)

type streamingClient struct {
	opts options

	state    *uint32
	conns    []*streamConn
	counter  *uint64
	validate validator

	crp *connRetryPool

	pool bytePool

	cache *bigcache.BigCache
}

func newStreamingClient(opts options) *streamingClient {
	if opts.maxStreams <= 0 {
		panic(fmt.Errorf("streaming client must be created with at least 1 stream but got %d", opts.maxStreams))
	}

	state := scsDisconnected
	counter := uint64(0)

	c := &streamingClient{
		opts:    opts,
		state:   &state,
		counter: &counter,
	}

	if !opts.autoRequestSize {
		c.pool = makeBytePool(int(opts.maxRequestSize), opts.noPool)
	}

	return c
}

func (c *streamingClient) Connect(addr string) error {
	if !atomic.CompareAndSwapUint32(c.state, scsDisconnected, scsConnecting) {
		return ErrorConnected
	}

	exitState := scsDisconnected
	defer func() { atomic.StoreUint32(c.state, exitState) }()

	addrs := c.opts.addresses
	c.validate = c.makeSimpleValidator()
	if len(addrs) > 1 {
		switch c.opts.balancer {
		default:
			panic(fmt.Errorf("invalid balancer %d", c.opts.balancer))

		case roundRobinBalancer:
			c.validate = c.makeRoundRobinValidator()

		case hotSpotBalancer:
			c.validate = c.makeHotSpotValidator()
		}
	} else if len(addrs) < 1 {
		addrs = []string{addr}
	}

	cache, err := newCacheFromOptions(c.opts)
	if err != nil {
		return err
	}

	conns, crp := makeStreamConns(c.opts.ctx, addrs, c.opts.maxStreams,
		c.opts.tracer, c.opts.connTimeout, c.opts.connStateCb)
	c.conns = conns
	c.crp = crp
	c.cache = cache

	exitState = scsConnected
	return nil
}

func (c *streamingClient) Close() {
	if !atomic.CompareAndSwapUint32(c.state, scsConnected, scsClosing) {
		return
	}

	c.crp.stop()
	closeStreamConns(c.conns)

	if c.cache != nil {
		c.cache.Reset()
		c.cache = nil
	}

	atomic.StoreUint32(c.state, scsClosed)
}

func (c *streamingClient) Validate(in, out interface{}) error {
	var (
		m   pb.Msg
		err error
	)

	if c.opts.autoRequestSize {
		m, err = makeRequest(in)
	} else {
		var b []byte
		switch in.(type) {
		default:
			b = c.pool.Get()
			defer c.pool.Put(b)

		case []byte, pb.Msg, *pb.Msg:
		}

		m, err = makeRequestWithBuffer(in, b)
	}
	if err != nil {
		return err
	}

	if c.cache != nil {
		var b []byte
		if b, err = c.cache.Get(string(m.Body)); err == nil {
			err = fillResponse(pb.Msg{Body: b}, out)
			if c.opts.onCacheHitHandler != nil {
				if err != nil {
					c.opts.onCacheHitHandler.Handle(in, b, err)
				} else {
					c.opts.onCacheHitHandler.Handle(in, out, nil)
				}
			}
			return err
		}
	}

	for atomic.LoadUint32(c.state) == scsConnected {
		if !c.crp.check() {
			c.crp.tryStart()
			if !c.crp.wait() {
				return ErrorNotConnected
			}
		}

		for i := 0; i < len(c.conns); i++ {
			r, err := c.validate(&m)
			if err == nil {
				if c.cache != nil {
					c.cache.Set(string(m.Body), r.Body)
				}

				return fillResponse(r, out)
			}

			if err != errConnFailure &&
				err != errStreamFailure &&
				err != errStreamConnWrongState &&
				err != errStreamWrongState {
				return err
			}
		}
	}

	return ErrorNotConnected
}

func (c *streamingClient) makeSimpleValidator() validator {
	return func(m *pb.Msg) (pb.Msg, error) {
		conn := c.conns[0]
		r, err := conn.validate(m)
		if err == errConnFailure {
			c.crp.put(conn)
		}

		return r, err
	}
}

func (c *streamingClient) makeRoundRobinValidator() validator {
	return func(m *pb.Msg) (pb.Msg, error) {
		i := int((atomic.AddUint64(c.counter, 1) - 1) % uint64(len(c.conns)))
		conn := c.conns[i]
		r, err := conn.validate(m)
		if err == errConnFailure {
			c.crp.put(conn)
		}

		return r, err
	}
}

func (c *streamingClient) makeHotSpotValidator() validator {
	return func(m *pb.Msg) (pb.Msg, error) {
		total := uint64(len(c.conns))
		start := atomic.LoadUint64(c.counter)
		i := int(start % total)
		for {
			conn := c.conns[i]
			r, ok, err := conn.tryValidate(m)
			if ok {
				if err == errConnFailure {
					c.crp.put(conn)
				}

				return r, err
			}

			new := atomic.AddUint64(c.counter, 1)
			if new-start >= total {
				break
			}

			i = int(new % total)
		}

		conn := c.conns[i]
		r, err := conn.validate(m)
		if err == errConnFailure {
			c.crp.put(conn)
		}

		return r, err
	}
}
