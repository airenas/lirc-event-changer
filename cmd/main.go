package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/airenas/lirc-event-changer/internal/pkg/lirc"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type writers interface {
	GetWriters() []chan<- string
}

type event struct {
	id, name, de string
	repeat       int
}

type wdata struct {
	event  *event
	repeat int
	at     time.Time
	last   time.Time
}

type allData struct {
	data   []*wdata
	lock   *sync.Mutex
	waitCh chan bool
}

func parseEvent(str string) (*event, error) {
	strs := strings.Split(str, " ")
	if len(strs) < 4 {
		return nil, errors.Errorf("wrong event %s", str)
	}
	res := &event{}
	res.id = strs[0]
	res.name = strs[2]
	res.de = strs[3]
	rep, err := strconv.ParseInt(strs[1], 16, 32)
	res.repeat = int(rep)
	if err != nil {
		return nil, errors.Errorf("can't parse hex %s", strs[1])
	}

	return res, nil
}

func toString(d *wdata) string {
	h := ""
	if d.last.After(d.at.Add(340 * time.Millisecond)) {
		h = "_HOLD"
	}
	return fmt.Sprintf("%s %x %s %s", d.event.id, 0, d.event.name+h, d.event.de)
}

func (data *allData) addEvent(e *event, t time.Time) {
	data.lock.Lock()
	if len(data.data) == 0 {
		if (e.repeat == 0) {
			data.data = append(data.data, &wdata{event: e, at: t, last: t, repeat: 0})
		} else {
			logrus.Debug("Skip repeat")
		}
	} else {
		d := data.data[len(data.data)-1]
		if d.event.name != e.name {
			logrus.Debug("Name differs")
			data.data = append(data.data, &wdata{event: e, at: t, last: t, repeat: 0})
		} else if d.repeat != e.repeat-1 {
			logrus.Debugf("Repeat differs %d, %d", d.repeat, e.repeat)
			data.data = append(data.data, &wdata{event: e, at: t, last: t, repeat: 0})
		} else if needSend(d, t) {
			data.data = append(data.data, &wdata{event: e, at: t, last: t, repeat: 0})
		} else {
			d.repeat++
			d.last = t
		}
	}
	defer data.lock.Unlock()
	data.waitCh <- true
}

func needSend(w *wdata, t time.Time) bool {
	if w.last.Before(t.Add(-100 * time.Millisecond)) {
		logrus.Debugf("Skip hold - too long period %d, %d", w.last.UnixNano(), t.UnixNano())
		return true
	}
	if w.at.Before(t.Add(-500 * time.Millisecond)) {
		logrus.Debugf("Add hold - long enough %d, %d", w.last.UnixNano(), t.UnixNano())
		return true
	}
	logrus.Debugf("Maybe hold - %d, %d", w.last.UnixNano(), t.UnixNano())
	return false
}

func listen(r <-chan string, data *allData, closeChannel chan bool) {
	for {
		select {
		case <-closeChannel:
			logrus.Info("Exit main loop")
			return
		case str := <-r:
			e, err := parseEvent(str)
			if err != nil {
				logrus.Error(errors.Wrapf(err, "can't parse %s", str))
				continue
			}
			data.addEvent(e, time.Now())
		}
	}
}

func onTimer(w chan<- string, data *allData, closeChannel chan bool) {
	for {
		select {
		case <-closeChannel:
			logrus.Debug("Exit time loop")
			return
		case <-time.After(time.Millisecond * 20):
			logrus.Debug("On timer")
			data.lock.Lock()
			t := time.Now()
			l := len(data.data)
			logrus.Debugf("Len %d, time %d", l, t.UnixNano())
			if l == 0 {
				data.lock.Unlock()
				logrus.Debug("Wait for events")
				select {
				case <-data.waitCh:
				case <-closeChannel:
					logrus.Info("Exit time loop")
					return
				}
				continue
			}
			for i, d := range data.data {
				if i < (l - 1) {
					logrus.Debug("Send")
					w <- toString(d)
				} else {
					data.data[0] = d
				}
			}
			if needSend(data.data[0], t) {
				logrus.Debug("Send")
				w <- toString(data.data[0])
				data.data = data.data[:0]
			} else {
				data.data = data.data[:1]
			}
			data.lock.Unlock()
		}
	}
}

func pushChannel(wr writers, closeChannel chan bool) chan<- string {
	res := make(chan string, 2)
	go func() {
		for {
			select {
			case str := <-res:
				for _, ch := range wr.GetWriters() {
					select {
					case ch <- str:
					case <-time.After(time.Millisecond * 20):
						logrus.Error(errors.New("write timeout"))
					}
				}
			case <-closeChannel:
				logrus.Info("Exit write loop")
				return
			}
		}
	}()
	return res
}

func pullChannel(r io.Reader, closeChannel chan bool, errChannel chan<- error) <-chan string {
	res := make(chan string, 2)
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := r.Read(buf[:])
			if err != nil {
				errChannel <- errors.Wrapf(err, "Can't read")
				return
			}
			logrus.Info(strings.TrimSpace(string(buf[0:n])))
			select {
			case res <- string(buf[0:n]):
			case <-closeChannel:
				logrus.Info("Exit read loop")
				return
			}
		}
	}()
	return res
}

func main() {
	logrus.SetFormatter(&logrus.TextFormatter{TimestampFormat: "2006-01-02 15:04:05.000", FullTimestamp: true})
	logrus.Info("Starting lirc changer")

	pathIn := "/var/run/lirc/lircd"
	logrus.Infof("Listen socket: %s", pathIn)
	sIn, err := net.Dial("unix", pathIn)
	if err != nil {
		panic(errors.Wrapf(err, "Can't open %s", pathIn))
	}
	defer sIn.Close()

	pathOut := "/var/run/lirc/lircd1"
	cc := make(chan error, 2)
	closeChan := make(chan bool, 2)

	wData := lirc.NewData()

	wrCh := pushChannel(wData, closeChan)
	rdCh := pullChannel(sIn, closeChan, cc)

	data := newAllData()
	go listen(rdCh, data, closeChan)
	go onTimer(wrCh, data, closeChan)

	ln, err := lirc.NewSocket(pathOut)
	if err != nil {
		panic(err)
	}
	defer ln.Close()

	go lirc.StartService(ln, wData, cc, closeChan)
	/////////////////////// Waiting for terminate
	wc := make(chan os.Signal, 2)
	signal.Notify(wc, os.Interrupt, syscall.SIGTERM)
	select {
	case <-wc:
	case err := <-cc:
		panic(errors.Wrapf(err, "Listen error"))
	}
	close(closeChan)
	logrus.Info("Exiting the lirc changer")
	/////////////////////////////////////////////
}

func newAllData() *allData {
	res := &allData{}
	res.data = make([]*wdata, 0)
	res.lock = &sync.Mutex{}
	res.waitCh = make(chan bool, 100)
	return res
}
