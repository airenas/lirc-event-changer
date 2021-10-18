package lirc

import (
	"context"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type writers struct {
	wArray []chan<- string
	lock   *sync.Mutex
	wg     *sync.WaitGroup
}

func newData() *writers {
	res := writers{}
	res.wArray = make([]chan<- string, 0)
	res.lock = &sync.Mutex{}
	res.wg = &sync.WaitGroup{}
	return &res
}

func (w *writers) getWriters() []chan<- string {
	w.lock.Lock()
	defer w.lock.Unlock()
	return w.wArray
}

func (w *writers) add(ch chan<- string) {
	w.lock.Lock()
	defer w.lock.Unlock()
	w.wg.Add(1)
	w.wArray = append(w.wArray, ch)
	logrus.Infof("Add conn. Total %d", len(w.wArray))
}

func (w *writers) remove(ch chan<- string) {
	w.lock.Lock()
	defer w.lock.Unlock()
	defer w.wg.Done()
	cp := w.wArray
	w.wArray = nil
	for _, cht := range cp {
		if ch != cht {
			w.wArray = append(w.wArray, cht)
		}
	}
	logrus.Infof("Drop conn. Remaining %d", len(w.wArray))
}

func StartService(ctx context.Context, l net.Listener) (chan<- string, func() <-chan struct{}) {
	data := newData()
	go func() {
		for {
			con, err := l.Accept()
			if err != nil {
				select {
				case <-ctx.Done():
					return
				default:
					logrus.Error(errors.Wrap(err, "can't accept connection"))
					continue
				}
			}
			logrus.Infof("New connection %s", con.LocalAddr().String())
			go listen(ctx, con, data)
		}
	}()
	return pushChannel(ctx, data), func() <-chan struct{} { return doneFunc(data) }
}

func doneFunc(wr *writers) <-chan struct{} {
	res := make(chan struct{})
	go func() {
		wr.wg.Wait()
		close(res)
	}()
	return res
}

func pushChannel(ctx context.Context, wr *writers) chan<- string {
	res := make(chan string, 2)
	go func() {
		for {
			select {
			case str := <-res:
				for _, ch := range wr.getWriters() {
					select {
					case ch <- str:
					case <-time.After(time.Millisecond * 20):
						logrus.Error(errors.New("write timeout"))
					}
				}
			case <-ctx.Done():
				logrus.Info("Exit write loop")
				return
			}
		}
	}()
	return res
}

func listen(ctx context.Context, con io.WriteCloser, data *writers) {
	defer con.Close()
	wrCh := make(chan string, 2)
	data.add(wrCh)
	defer data.remove(wrCh)
	for {
		select {
		case s := <-wrCh:
			n, err := con.Write([]byte(s))
			if err != nil {
				logrus.Error(errors.Wrapf(err, "can't write '%s'", strings.TrimSpace(s)))
				return
			}
			logrus.Infof("Wrote %d bytes", n)
		case <-ctx.Done():
			return
		}
	}
}
