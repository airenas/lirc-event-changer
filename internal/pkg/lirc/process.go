package lirc

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type Event struct {
	id, name, device string
	repeat           int
	at               time.Time
}

func parseEvent(str string) (*Event, error) {
	strs := strings.Split(str, " ")
	if len(strs) < 4 {
		return nil, errors.Errorf("wrong event %s", str)
	}
	res := &Event{}
	res.id = strs[0]
	res.name = strs[2]
	res.device = strs[3]
	rep, err := strconv.ParseInt(strs[1], 16, 32)
	res.repeat = int(rep)
	if err != nil {
		return nil, errors.Errorf("can't parse hex %s", strs[1])
	}
	return res, nil
}

func NewPuller(ctx context.Context, r io.Reader, errCh chan<- error) <-chan string {
	logrus.Infof("Init pooler")
	res := make(chan string, 2)
	clInnerCh := make(chan bool)
	go func() {
		defer close(clInnerCh)
		buf := make([]byte, 1024)
		for {
			n, err := r.Read(buf[:])
			if err != nil {
				errCh <- errors.Wrapf(err, "can't read")
				return
			}
			res <- string(buf[0:n])
		}
	}()
	go func() {
		defer close(res)
		select {
		case <-ctx.Done():
		case <-clInnerCh:
		}
		logrus.Infof("Exit pooler")
	}()
	return res
}

func NewLogger(in <-chan string, prefix string) <-chan string {
	logrus.Infof("Init logger %s...", prefix)
	res := make(chan string, 2)
	go func() {
		defer close(res)
		for {
			d, ok := <-in
			if !ok {
				logrus.Infof("Exit logger %s...", prefix)
				return
			}
			logrus.Infof("%s%s", prefix, d)
			res <- d
		}
	}()
	return res
}

func NewParser(in <-chan string) <-chan *Event {
	logrus.Infof("Init parser")
	res := make(chan *Event, 2)
	go func() {
		defer close(res)
		for {
			d, ok := <-in
			if !ok {
				logrus.Infof("Exit parser")
				return
			}
			e, err := parseEvent(d)
			if err != nil {
				logrus.Error(errors.Wrapf(err, "can't parse %s", d))
				continue
			}
			res <- e
		}
	}()
	return res
}

func toString(d *Event) string {
	h := ""
	// if d.last.After(d.at.Add(340 * time.Millisecond)) {
	// 	h = "_HOLD"
	// }
	return fmt.Sprintf("%s %x %s %s", d.id, 0, d.name+h, d.device)
}

func ToString(in <-chan *Event) <-chan string {
	logrus.Infof("Init Stringer")
	res := make(chan string, 2)
	go func() {
		defer close(res)
		for {
			d, ok := <-in
			if !ok {
				logrus.Infof("Exit stringer")
				return
			}
			res <- toString(d)
		}
	}()
	return res
}

func Mapper(in <-chan *Event) <-chan *Event {
	logrus.Infof("Init mapper")
	res := make(chan *Event, 2)
	go func() {
		defer close(res)
		var ev *Event
		var timerCh <-chan time.Time

		addEvent := func(d *Event, t time.Time, after time.Duration) {
			ev = d
			ev.at = t
			timerCh = time.After(after)
		}

		for {
			select {
			case d, ok := <-in:
				timerCh = nil
				t := time.Now()
				if !ok {
					logrus.Infof("Exit mapper")
					return
				}
				if ev == nil {
					if d.repeat == 0 {
						addEvent(d, t, time.Millisecond*100)
					} else {
						logrus.Debug("Skip repeat")
					}
				} else {
					if d.name != ev.name {
						logrus.Debug("Name differs")
						res <- ev
						addEvent(d, t, time.Millisecond*100)
					} else if ev.repeat != d.repeat-1 {
						res <- ev
						if (d.repeat == 0) {
							addEvent(d, t, time.Millisecond*100)
						}
					} else if ev.at.Before(t.Add(-500 * time.Millisecond)) {
						ev.name += "_HOLD"
						res <- ev
						ev = nil
					} else {
						ev.repeat++
						timerCh = time.After(time.Millisecond * 100)
					}
				}
			case <-timerCh:
				timerCh = nil
				if ev != nil {
					t := time.Now()
					if ev.at.Before(t.Add(-500 * time.Millisecond)) {
						ev.name += "_HOLD"
					}
					res <- ev
				}
				ev = nil
			}
		}
	}()
	return res
}
