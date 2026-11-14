// Package worker talks to the Python model worker over a Unix socket
// (08/P3, D32): newline-delimited JSON, with progress lines for long jobs.
package worker

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

type Client struct {
	sock string

	mu      sync.Mutex
	conn    net.Conn
	hello   json.RawMessage
	next    int64
	pending map[int64]*call
	subs    map[chan struct{}]struct{}
}

type call struct {
	reply    chan json.RawMessage
	progress func(done, total int)
}

func New(sock string) *Client {
	return &Client{sock: sock, pending: map[int64]*call{}, subs: map[chan struct{}]struct{}{}}
}

// ErrDown means no worker is connected.
var ErrDown = errors.New("model worker is not connected")

// Run keeps a connection to the worker, reconnecting when it drops.
func (c *Client) Run(ctx context.Context) {
	wait := time.Second
	for ctx.Err() == nil {
		conn, err := net.Dial("unix", c.sock)
		if err != nil {
			time.Sleep(wait)
			if wait < 10*time.Second {
				wait *= 2
			}
			continue
		}
		wait = time.Second
		c.serve(conn)
	}
}

func (c *Client) serve(conn net.Conn) {
	r := bufio.NewReaderSize(conn, 1<<20)
	first, err := r.ReadBytes('\n')
	if err != nil {
		conn.Close()
		return
	}
	c.mu.Lock()
	c.conn, c.hello = conn, json.RawMessage(first)
	c.mu.Unlock()
	c.notify()
	log.Printf("worker connected")
	for {
		line, err := r.ReadBytes('\n')
		if err != nil {
			break
		}
		var head struct {
			ID       *int64 `json:"id"`
			Progress []int  `json:"progress"`
			Op       string `json:"op"`
		}
		if json.Unmarshal(line, &head) != nil {
			continue
		}
		if head.Op == "hello" {
			c.mu.Lock()
			c.hello = json.RawMessage(line)
			c.mu.Unlock()
			c.notify()
			continue
		}
		if head.ID == nil {
			continue
		}
		c.mu.Lock()
		cl := c.pending[*head.ID]
		if cl != nil && head.Progress == nil {
			delete(c.pending, *head.ID)
		}
		c.mu.Unlock()
		if cl == nil {
			continue
		}
		if head.Progress != nil {
			if cl.progress != nil && len(head.Progress) == 2 {
				cl.progress(head.Progress[0], head.Progress[1])
			}
			continue
		}
		cl.reply <- json.RawMessage(line)
	}
	conn.Close()
	c.mu.Lock()
	c.conn = nil
	for id, cl := range c.pending {
		close(cl.reply)
		delete(c.pending, id)
	}
	c.mu.Unlock()
	c.notify()
	log.Printf("worker disconnected")
}

// Hello returns the worker's identity message, or nil when down.
func (c *Client) Hello() json.RawMessage {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return nil
	}
	return c.hello
}

// Watch returns a channel that receives a value whenever the connection state
// or identity changes, and a function to stop watching.
func (c *Client) Watch() (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	c.mu.Lock()
	c.subs[ch] = struct{}{}
	c.mu.Unlock()
	return ch, func() {
		c.mu.Lock()
		delete(c.subs, ch)
		c.mu.Unlock()
	}
}

func (c *Client) notify() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for ch := range c.subs {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// Call sends a request and waits for its reply. req must be a JSON object;
// the id is added here. A reply with ok=false becomes an error.
func (c *Client) Call(ctx context.Context, req map[string]any, progress func(done, total int)) (json.RawMessage, error) {
	c.mu.Lock()
	if c.conn == nil {
		c.mu.Unlock()
		return nil, ErrDown
	}
	c.next++
	id := c.next
	cl := &call{reply: make(chan json.RawMessage, 1), progress: progress}
	c.pending[id] = cl
	conn := c.conn
	req["id"] = id
	b, err := json.Marshal(req)
	if err == nil {
		_, err = conn.Write(append(b, '\n'))
	}
	c.mu.Unlock()
	if err != nil {
		return nil, err
	}
	select {
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, ctx.Err()
	case rep, ok := <-cl.reply:
		if !ok {
			return nil, ErrDown
		}
		var st struct {
			OK    bool   `json:"ok"`
			Error string `json:"error"`
		}
		json.Unmarshal(rep, &st)
		if !st.OK {
			return nil, fmt.Errorf("worker: %s", st.Error)
		}
		return rep, nil
	}
}
