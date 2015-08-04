// Copyright (c) 2015, Air Computing Inc. <oss@aerofs.com>
// All rights reserved.

package main

import (
	"io"
	"os"
	"os/signal"
	"runtime/pprof"
	"strconv"
	"syscall"
	"time"
)

type StatsDumper interface {
	DumpStats(w io.Writer)
}

func SetupSignalHandler(sd StatsDumper) {
	c := make(chan os.Signal, 10)
	go signalLoop(c, sd)
	signal.Notify(c,
		syscall.SIGUSR1,
		syscall.SIGUSR2,
	)
}

func signalLoop(c chan os.Signal, sd StatsDumper) {
	for s := range c {
		ts := strconv.FormatInt(time.Now().Unix(), 16)
		switch s.(syscall.Signal) {
		case syscall.SIGUSR1:
			sd.DumpStats(os.Stdout)
		case syscall.SIGUSR2:
			if f, err := os.Create("heap-" + ts); err == nil {
				pprof.WriteHeapProfile(f)
			}
		}
	}
}
