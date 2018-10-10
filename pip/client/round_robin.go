package client

import (
	"sync"
	"sync/atomic"
)

type roundRobinBalancer struct {
	sync.RWMutex

	idx   *uint64
	conns []*connection
}

func (b *roundRobinBalancer) start(c *client) error {
	addrs := c.opts.addrs

	var err error
	if len(addrs) <= 0 {
		addrs, err = lookupHostPort(c.opts.addr)
		if err != nil {
			return err
		}
	}

	nets, err := dialTimeout(c.opts.net, addrs, c.opts.connTimeout)
	if err != nil {
		return err
	}

	conns := make([]*connection, len(nets))
	for i, n := range nets {
		conns[i] = c.newConnection(n)
		conns[i].start()
	}

	b.Lock()
	b.idx = new(uint64)
	b.conns = conns
	b.Unlock()

	return nil
}

func (b *roundRobinBalancer) stop() {
	b.Lock()
	conns := b.conns
	b.conns = nil
	b.idx = nil
	b.Unlock()

	wg := new(sync.WaitGroup)
	for _, c := range conns {
		if c != nil {
			wg.Add(1)
			go func(c *connection) {
				defer wg.Done()
				c.close()
			}(c)
		}
	}

	wg.Wait()
}

func (b *roundRobinBalancer) get() *connection {
	b.RLock()
	defer b.RUnlock()

	idx := b.idx
	if idx == nil {
		return nil
	}

	conns := b.conns
	if conns == nil {
		return nil
	}

	i := int((atomic.AddUint64(b.idx, 1) - 1) % uint64(len(conns)))
	conn := conns[i]
	if conn != nil {
		conn.g.Add(1)
	}

	return conn
}
