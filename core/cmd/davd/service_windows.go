//go:build windows

package main

import (
	"fmt"
	"os"

	"davdeck.dev/davdeck/core/internal/platform"
	"golang.org/x/sys/windows/svc"
)

type windowsService struct {
	runDaemon func(<-chan os.Signal) error
}

func runPlatform() error {
	isService, err := svc.IsWindowsService()
	if err != nil {
		return fmt.Errorf("detect Windows service context: %w", err)
	}
	if !isService {
		return run()
	}
	if err := svc.Run(platform.ServiceName(), &windowsService{runDaemon: runDaemon}); err != nil {
		return fmt.Errorf("run Windows service: %w", err)
	}
	return nil
}

func (s *windowsService) Execute(_ []string, requests <-chan svc.ChangeRequest, statuses chan<- svc.Status) (bool, uint32) {
	if s.runDaemon == nil {
		return false, 1
	}

	stopChannel := make(chan os.Signal, 1)
	daemonErrors := make(chan error, 1)
	go func() { daemonErrors <- s.runDaemon(stopChannel) }()

	currentStatus := svc.Status{State: svc.StartPending}
	statuses <- currentStatus
	currentStatus = svc.Status{
		State:   svc.Running,
		Accepts: svc.AcceptStop | svc.AcceptShutdown,
	}
	statuses <- currentStatus

	for {
		select {
		case request, ok := <-requests:
			if !ok {
				return false, 1
			}
			switch request.Cmd {
			case svc.Interrogate:
				statuses <- currentStatus
			case svc.Stop, svc.Shutdown:
				currentStatus = svc.Status{State: svc.StopPending}
				statuses <- currentStatus
				stopChannel <- os.Interrupt
				if err := <-daemonErrors; err != nil {
					return false, 1
				}
				return false, 0
			}
		case err := <-daemonErrors:
			if err != nil {
				return false, 1
			}
			return false, 0
		}
	}
}
