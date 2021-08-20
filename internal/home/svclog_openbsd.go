//go:build openbsd
// +build openbsd

package home

import (
	"fmt"

	"github.com/AdguardTeam/golibs/log"
	"github.com/kardianos/service"
)

func newSysLogger(name string, errs chan<- error) (service.Logger, error) {
	return sysLogger(errs), nil
}

type sysLogger chan<- error

func (s sysLogger) Error(v ...interface{}) error {
	log.Error(fmt.Sprint(v...))

	return nil
}

func (s sysLogger) Warning(v ...interface{}) error {
	log.Info("warning: %s", fmt.Sprint(v...))

	return nil
}

func (s sysLogger) Info(v ...interface{}) error {
	log.Info(fmt.Sprint(v...))

	return nil
}

func (s sysLogger) Errorf(format string, a ...interface{}) error {
	log.Error(format, a...)

	return nil
}

func (s sysLogger) Warningf(format string, a ...interface{}) error {
	log.Info("warning: %s", fmt.Sprintf(format, a...))

	return nil
}

func (s sysLogger) Infof(format string, a ...interface{}) error {
	log.Info(format, a...)

	return nil
}
