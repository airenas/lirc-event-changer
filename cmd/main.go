package main

import (
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/airenas/lirc-event-changer/internal/pkg/lirc"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func main() {
	pathIn := "/var/run/lirc/lircd"
	pathOut := "/var/run/lirc/lircd1"

	logrus.SetFormatter(&logrus.TextFormatter{TimestampFormat: "2006-01-02 15:04:05.000", FullTimestamp: true})
	logrus.Info("Starting lirc changer")

	/////////////////////////////////////////////////////////////
	logrus.Infof("Listen socket: %s", pathIn)
	sIn, err := net.Dial("unix", pathIn)
	if err != nil {
		panic(errors.Wrapf(err, "Can't open %s", pathIn))
	}
	defer sIn.Close()

	/////////////////////////////////////////////////////////////
	ln, err := lirc.NewSocket(pathOut)
	if err != nil {
		panic(err)
	}
	defer ln.Close()
	
	/////////////////////////////////////////////////////////////
	errCh := make(chan error, 2)
	closeCh := make(chan bool)
	pullCh := lirc.NewPuller(sIn, closeCh, errCh)
	outCh := lirc.NewLogger(lirc.ToString(lirc.Mapper(lirc.NewParser(lirc.NewLogger(pullCh, "In: ")))), "Out: ")
	writeCh := lirc.StartService(ln, closeCh)
	go passData(outCh, writeCh)
	/////////////////////////////////////////////////////////////

	/////////////////////// Waiting for terminate
	waitCh := make(chan os.Signal, 2)
	signal.Notify(waitCh, os.Interrupt, syscall.SIGTERM)
	select {
	case <-waitCh:
		logrus.Info("Got exit signal")
	case err := <-errCh:
		panic(errors.Wrapf(err, "Listen error"))
	}
	close(closeCh)
	select{
		case 
	}
	time.Sleep(time.Second)
	logrus.Info("Exiting the lirc changer")
}

func passData(in <-chan string, out chan<- string) {
	logrus.Infof("Init data pass")
	for {
		d, ok := <-in
		if !ok {
			logrus.Infof("Exit data pass")
			return
		}
		out <- d
	}

}
