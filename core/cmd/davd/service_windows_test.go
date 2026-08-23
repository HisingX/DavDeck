//go:build windows

package main

import (
	"errors"
	"os"
	"testing"
	"time"

	"golang.org/x/sys/windows/svc"
)

func TestWindowsServiceStopsDaemonOnStopRequest(t *testing.T) {
	stopReceived := make(chan struct{})
	service := &windowsService{
		runDaemon: func(stopChannel <-chan os.Signal) error {
			select {
			case <-stopChannel:
				close(stopReceived)
				return nil
			case <-time.After(time.Second):
				return errors.New("timed out waiting for stop signal")
			}
		},
	}
	requests := make(chan svc.ChangeRequest, 1)
	statuses := make(chan svc.Status, 4)
	result := make(chan struct {
		specific bool
		code     uint32
	}, 1)
	go func() {
		specific, code := service.Execute(nil, requests, statuses)
		result <- struct {
			specific bool
			code     uint32
		}{specific: specific, code: code}
	}()

	if status := waitForServiceStatus(t, statuses); status.State != svc.StartPending {
		t.Fatalf("start status = %v, want %v", status.State, svc.StartPending)
	}
	if status := waitForServiceStatus(t, statuses); status.State != svc.Running {
		t.Fatalf("running status = %v, want %v", status.State, svc.Running)
	}
	requests <- svc.ChangeRequest{Cmd: svc.Stop}
	if status := waitForServiceStatus(t, statuses); status.State != svc.StopPending {
		t.Fatalf("stop status = %v, want %v", status.State, svc.StopPending)
	}

	select {
	case <-stopReceived:
	case <-time.After(time.Second):
		t.Fatal("daemon did not receive stop signal")
	}
	select {
	case execution := <-result:
		if execution.specific || execution.code != 0 {
			t.Fatalf("service execution = specific %v, code %d; want false, 0", execution.specific, execution.code)
		}
	case <-time.After(time.Second):
		t.Fatal("service handler did not exit")
	}
}

func waitForServiceStatus(t *testing.T, statuses <-chan svc.Status) svc.Status {
	t.Helper()
	select {
	case status := <-statuses:
		return status
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for service status")
		return svc.Status{}
	}
}
