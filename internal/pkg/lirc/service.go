package lirc

import (
	"io"
	"net"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type Writers struct {
	wArray  []chan<- string
	lock    *sync.Mutex
}

func NewData() *Writers {
	res := Writers{}
	res.wArray = make([]chan <-string,0)
	res.lock = &sync.Mutex{}
	return &res
}

func (w *Writers) GetWriters() []chan<- string {
	w.lock.Lock()
	defer w.lock.Unlock()
	return w.wArray
}

func (w *Writers) Add(ch chan<- string) {
	w.lock.Lock()
	defer w.lock.Unlock()
	w.wArray = append(w.wArray, ch)
	logrus.Infof("Add conn. Total %d", len(w.wArray))
}

func (w *Writers) Remove(ch chan<- string) {
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

func StartService(l net.Listener, data *Writers, errCh chan<- error, closeCh <-chan bool) {
	for {
		con, err := l.Accept()
		if err != nil {
			errCh <- errors.Wrap(err, "can't accept connection")
			break
		}
		logrus.Infof("New connection %s", con.LocalAddr().String())
		go listen(con, data, closeCh)
	}
}

func listen(con io.WriteCloser, data *Writers, closeChan <-chan bool) {
	defer con.Close()
	wrCh := make(chan string, 2)
	data.Add(wrCh)
	defer data.Remove(wrCh)
	for {
		select {
		case s := <-wrCh:
			n, err := con.Write([]byte(s))
			if err != nil {
				logrus.Error(errors.Wrapf(err, "can't write '%s'", strings.TrimSpace(s)))
				return
			}
			logrus.Infof("Wrote %d bytes", n)
		case <-closeChan:
			return
		}
	}
}
