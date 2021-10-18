package lirc

import (
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
}

func newData() *writers {
	res := writers{}
	res.wArray = make([]chan<- string, 0)
	res.lock = &sync.Mutex{}
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
	w.wArray = append(w.wArray, ch)
	logrus.Infof("Add conn. Total %d", len(w.wArray))
}

func (w *writers) remove(ch chan<- string) {
	w.lock.Lock()
	defer w.lock.Unlock()
	cp := w.wArray
	w.wArray = nil
	for _, cht := range cp {
		if ch != cht {
			w.wArray = append(w.wArray, cht)
		}
	}
	logrus.Infof("Drop conn. Remaining %d", len(w.wArray))
}

func StartService(l net.Listener, closeCh <-chan bool) chan<- string {
	data := newData()
	go func () {
		for {
			con, err := l.Accept()
			if err != nil {
				logrus.Error(errors.Wrap(err, "can't accept connection"))
				continue					
			}
			logrus.Infof("New connection %s", con.LocalAddr().String())
			go listen(con, data, closeCh)
		}
	} ()
	return pushChannel(data, closeCh)
}

func pushChannel(wr *writers, closeCh <- chan bool) chan<- string {
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
			case <-closeCh:
				logrus.Info("Exit write loop")
				return
			}
		}
	}()
	return res
}

func listen(con io.WriteCloser, data *writers, closeCh <-chan bool) {
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
		case <-closeCh:
			return
		}
	}
}
