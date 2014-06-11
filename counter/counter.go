package counter

import (
	"sync"
)

type Counter struct {
	mu sync.Mutex
	x  int64
}

func (c *Counter) Add(x int64) {
	c.mu.Lock()
	c.x += x
	c.mu.Unlock()
}

func (c *Counter) Reset() {
	c.mu.Lock()
	c.x = 0
	c.mu.Unlock()
}

func (c *Counter) Value() (x int64) {
	c.mu.Lock()
	x = c.x
	c.mu.Unlock()
	return
}
