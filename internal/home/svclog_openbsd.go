//go:build openbsd
// +build openbsd

package home

import (
	"fmt"

	"github.com/AdguardTeam/golibs/log"
	"github.com/kardianos/service"
)

// newSysLogger returns a stub service.Logger implementation.
func newSysLogger(_ string, _ chan<- error) (service.Logger, error) {
	return sysLogger{}, nil
}

// sysLogger wraps calls of the logging functions understandable for service
// interfaces.
type sysLogger struct{}

// Error implements service.Logger interface for sysLogger.
func (sysLogger) Error(v ...interface{}) error {
	log.Error(fmt.Sprint(v...))

	return nil
}

// Warning implements service.Logger interface for sysLogger.
func (sysLogger) Warning(v ...interface{}) error {
	log.Info("warning: %s", fmt.Sprint(v...))

	return nil
}

// Info implements service.Logger interface for sysLogger.
func (sysLogger) Info(v ...interface{}) error {
	log.Info(fmt.Sprint(v...))

	return nil
}

// Errorf implements service.Logger interface for sysLogger.
func (sysLogger) Errorf(format string, a ...interface{}) error {
	log.Error(format, a...)

	return nil
}

// Warningf implements service.Logger interface for sysLogger.
func (sysLogger) Warningf(format string, a ...interface{}) error {
	log.Info("warning: %s", fmt.Sprintf(format, a...))

	return nil
}

// Infof implements service.Logger interface for sysLogger.
func (sysLogger) Infof(format string, a ...interface{}) error {
	log.Info(format, a...)

	return nil
}
