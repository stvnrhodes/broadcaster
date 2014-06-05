// Package broadcaster allows broadcasting a value over many channels at once.
package broadcaster

import (
	"sync"
	"time"
)

const (
	// Maximum time to try broadcasting to a subscriber.
	defaultWait = time.Second
)

// Options allows specifying additional options when creating a broadcaster.
type Options struct {
	// Time to wait on a subscriber's channel before giving up. Default is 1s.
	WaitTime time.Duration
}

func (o *Options) waitTime() time.Duration {
	if o == nil || o.WaitTime == 0 {
		return defaultWait
	}
	return o.WaitTime
}

// Caster allows sending messages to many subscribed channels.
type Caster interface {
	// Subscribe will return a channel that recieves broadcasted messages, along
	// with a done channel that unsubscribes and closes the broadcast channel
	// when closed. When unsubscribing, any messages currently pending will still
	// still be attempting to send until after the WaitTime.
	Subscribe(done <-chan struct{}) <-chan interface{}
	// Cast sends a message to be broadcast to all subscribers.
	Cast(msg interface{})
	// Close closes all created channels. Further calls to Close or Cast will
	// panic, and further subscriptions will return a closed channel.
	Close()
}

type subber struct {
	ch chan<- interface{}
	wg sync.WaitGroup
}

type broadcaster struct {
	subbers     []*subber
	cast        chan interface{}
	join, leave chan chan interface{}
	finish      chan struct{}
	waitTime    time.Duration
}

// New creates a new broadcaster that broadcasts a message to many subscribers.
// If multiple options are supplied, only the first one is considered.
func New(opts ...*Options) Caster {
	var o *Options
	if len(opts) > 0 {
		o = opts[0]
	}
	b := &broadcaster{
		cast:     make(chan interface{}),
		finish:   make(chan struct{}),
		join:     make(chan chan interface{}),
		leave:    make(chan chan interface{}),
		waitTime: o.waitTime(),
	}
	go b.run()
	return b
}

func (b *broadcaster) trySend(s *subber, m interface{}) {
	defer s.wg.Done()
	select {
	case <-time.After(b.waitTime):
	case s.ch <- m:
	}
}

func (b *broadcaster) run() {
	for {
		select {

		case ch := <-b.join:
			b.subbers = append(b.subbers, &subber{ch: ch})

		case ch := <-b.leave:
			for i, s := range b.subbers {
				if s.ch == ch {
					b.subbers = append(b.subbers[:i], b.subbers[i+1:]...)
					go func(s *subber) { s.wg.Wait(); close(s.ch) }(s)
					continue
				}
			}

		case m := <-b.cast:
			for _, s := range b.subbers {
				s.wg.Add(1)
				go b.trySend(s, m)
			}

		case <-b.finish:
			close(b.cast)
			for _, s := range b.subbers {
				go func(s *subber) { s.wg.Wait(); close(s.ch) }(s)
			}
			return

		}
	}
}

func (b *broadcaster) Subscribe(done <-chan struct{}) <-chan interface{} {
	ch := make(chan interface{})
	select {
	case b.join <- ch:
		go func() {
			<-done
			select {
			case b.leave <- ch:
			case <-b.finish:
			}
		}()
	case <-b.finish:
		close(ch)
	}
	return ch
}

func (b *broadcaster) Cast(msg interface{}) { b.cast <- msg }
func (b *broadcaster) Close()               { close(b.finish) }
