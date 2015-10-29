package broadcaster_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"stvn.cc/broadcaster"
)

func ExampleCaster() {
	// Create a new broadcaster
	b := broadcaster.New()

	// Create a channel subscribed to the broadcaster
	done := make(chan struct{})
	ch := b.Subscribe(done)

	// Wait groups are added to make the example more deterministic.
	// In most cases, having no wait groups is fine.
	var wg, wg2 sync.WaitGroup
	wg.Add(1)
	wg2.Add(1)

	// Read all messages from the channel.
	go func() {
		for msg := range ch {
			fmt.Println("Hello", msg)
		}
		wg.Done()
	}()

	b.Cast("World")

	// Create another channel. This one won't unsubscribe, so the argument is nil.
	ch2 := b.Subscribe(nil)

	// Read all messages from the channel.
	go func() {
		for msg := range ch2 {
			fmt.Println("Goodbye", msg)
		}
		wg2.Done()
	}()

	// Cast to the channel
	b.Cast(123)
	close(done)
	wg.Wait()
	b.Cast("examples")
	b.Close()
	wg2.Wait()
	// Output:
	// Hello World
	// Hello 123
	// Goodbye 123
	// Goodbye examples
}

func runAndCheck(t *testing.T, ch <-chan interface{}, length int, wg *sync.WaitGroup) {
	i := 0
	for _ = range ch {
		i++
	}
	if i != length {
		t.Errorf("Wrong number of messages to subscriber: got %v, want %v", i, length)
	}
	wg.Done()
}

func TestSubscribeOne(t *testing.T) {
	b := broadcaster.New()
	var wg sync.WaitGroup

	wg.Add(1)
	ch := b.Subscribe(nil)
	go runAndCheck(t, ch, 3, &wg)

	b.Cast(struct{}{})
	b.Cast("test string")
	b.Cast(123)
	b.Close()
	wg.Wait()
}

func TestSubscribeMany(t *testing.T) {
	b := broadcaster.New()
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		ch := b.Subscribe(nil)
		go runAndCheck(t, ch, 3, &wg)
	}

	b.Cast(struct{}{})
	b.Cast("test string")
	b.Cast(123)
	b.Close()
	wg.Wait()
}

func TestUnsubscribeOne(t *testing.T) {
	b := broadcaster.New()
	var wg sync.WaitGroup

	done := make(chan struct{})
	wg.Add(1)
	ch := b.Subscribe(done)
	go runAndCheck(t, ch, 3, &wg)

	b.Cast(struct{}{})
	b.Cast("test string")
	b.Cast(123)
	close(done)
	wg.Wait()
	b.Close()
}

func TestUnsubscribeMany(t *testing.T) {
	b := broadcaster.New()
	var wg1, wg2, wg3 sync.WaitGroup

	done1 := make(chan struct{})
	for i := 0; i < 5; i++ {
		wg1.Add(1)
		ch := b.Subscribe(done1)
		go runAndCheck(t, ch, 3, &wg1)
	}
	done2 := make(chan struct{})
	for i := 0; i < 5; i++ {
		wg2.Add(1)
		ch := b.Subscribe(done2)
		go runAndCheck(t, ch, 4, &wg2)
	}
	for i := 0; i < 5; i++ {
		wg3.Add(1)
		ch := b.Subscribe(nil)
		go runAndCheck(t, ch, 5, &wg3)
	}

	b.Cast(struct{}{})
	b.Cast("test string")
	b.Cast(123)
	close(done1)
	wg1.Wait()
	b.Cast(5.0)
	close(done2)
	wg2.Wait()
	b.Cast([]int{3, 5, 7})
	b.Close()
	wg3.Wait()
}

func expectPanic(t *testing.T, f func()) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic for %v", f)
		}
	}()
	f()
}

func TestAfterClose(t *testing.T) {
	b := broadcaster.New()
	var wg sync.WaitGroup

	wg.Add(1)
	ch := b.Subscribe(nil)
	go runAndCheck(t, ch, 0, &wg)

	b.Close()
	wg.Wait()

	done := make(chan struct{})
	closedCh := b.Subscribe(done)
	if v := <-closedCh; v != nil {
		t.Errorf("Unexpected non-nil value: %v", v)
	}
	close(done)
	if v := <-closedCh; v != nil {
		t.Errorf("Unexpected non-nil value: %v", v)
	}

	expectPanic(t, b.Close)
	expectPanic(t, func() { b.Cast(1) })
}

func BenchmarkSubscriberNum(b *testing.B) {
	caster := broadcaster.New()
	var wg sync.WaitGroup

	for i := 0; i < b.N; i++ {
		wg.Add(1)
		ch := caster.Subscribe(nil)
		go func() {
			for _ = range ch {
				// Empty the channel
			}
			wg.Done()
		}()
	}

	caster.Cast(struct{}{})
	caster.Cast("test string")
	caster.Cast(123)
	caster.Close()
	wg.Wait()
}

func Benchmark1Subscriber(b *testing.B)     { benchmarkNSubscribers(b, 1) }
func Benchmark10Subscribers(b *testing.B)   { benchmarkNSubscribers(b, 10) }
func Benchmark100Subscribers(b *testing.B)  { benchmarkNSubscribers(b, 100) }
func Benchmark1000Subscribers(b *testing.B) { benchmarkNSubscribers(b, 1000) }

func benchmarkNSubscribers(b *testing.B, n int) {
	caster := broadcaster.New(&broadcaster.Options{WaitTime: time.Hour})
	var wg sync.WaitGroup

	for i := 0; i < n; i++ {
		wg.Add(1)
		ch := caster.Subscribe(nil)
		go func() {
			for _ = range ch {
				// Empty the channel
			}
			wg.Done()
		}()
	}

	for i := 0; i < b.N; i++ {
		caster.Cast("test string")
	}
	caster.Close()
	wg.Wait()
}
