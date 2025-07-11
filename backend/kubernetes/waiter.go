package kubernetes

import (
	"context"
	"github.com/dns4acme/dns4acme/backend"
	"log/slog"
	"sync"
)

func newWaiter[T object[T]](lock sync.Locker, logAttrs ...slog.Attr) *waiter[T] {
	return &waiter[T]{
		refCount: map[string]int{},
		queues:   map[string]chan struct{}{},
		lock:     lock,
		logAttrs: logAttrs,
	}
}

type waiter[T object[T]] struct {
	refCount map[string]int
	queues   map[string]chan struct{}
	lock     sync.Locker
	logAttrs []slog.Attr
}

func (w waiter[T]) submit(object T) error {
	// Note: this is not locked because the object client is expected to lock beforehand.
	if _, ok := w.queues[object.name()]; !ok {
		return nil
	}
	close(w.queues[object.name()])
	return nil
}

func (w waiter[T]) wait(ctx context.Context, object T, condition func() (bool, error)) error { //nolint:unused // This is used, incorrectly reported
	name := object.name()
	w.lock.Lock()

	ok, err := condition()
	if err != nil {
		return err
	}
	if ok {
		w.lock.Unlock()
		return nil
	}

	for {
		waitChan, ok := w.queues[name]
		if !ok {
			waitChan = make(chan struct{})
			w.queues[name] = waitChan
			w.refCount[name] = 0
		}
		w.refCount[name]++
		w.lock.Unlock()
		select {
		case <-waitChan:
			w.lock.Lock()
			ok, err := condition()
			if err != nil {
				return err
			}
			if ok {
				w.refCount[name]--
				if w.refCount[name] == 0 {
					delete(w.queues, name)
					delete(w.refCount, name)
				}
				w.lock.Unlock()
				return nil
			}
			w.lock.Lock()
		case <-ctx.Done():
			err := backend.ErrBackendRequestFailed.
				Wrap(ctx.Err()).
				WithAttr(slog.String("name", object.name())).
				WithAttr(slog.String("reason", "timeout while waiting for Kubernetes backend to apply change"))
			for _, attr := range w.logAttrs {
				err = err.WithAttr(attr)
			}
			return err
		}
	}
}
