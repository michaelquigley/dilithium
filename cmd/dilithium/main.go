package main

import (
	"github.com/michaelquigley/pfxlog"
	_ "github.com/openziti/dilithium/cmd/dilithium/ctrl"
	"github.com/openziti/dilithium/cmd/dilithium/dilithium"
	_ "github.com/openziti/dilithium/cmd/dilithium/echo"
	_ "github.com/openziti/dilithium/cmd/dilithium/influx"
	_ "github.com/openziti/dilithium/cmd/dilithium/loop"
	_ "github.com/openziti/dilithium/cmd/dilithium/tunnel"
	"github.com/sirupsen/logrus"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
)

func init() {
	pfxlog.GlobalInit(logrus.InfoLevel, pfxlog.DefaultOptions().SetTrimPrefix("github.com/openziti/"))
}

func main() {
	go func() {
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGQUIT)
		buf := make([]byte, 1<<20)
		for {
			<-sigs
			stacklen := runtime.Stack(buf, true)
			log.Printf("=== received SIGQUIT ===\n*** goroutine dump...\n%s\n*** end\n", buf[:stacklen])
		}
	}()

	if err := dilithium.RootCmd.Execute(); err != nil {
		logrus.Fatalf("error (%v)", err)
	}
}
