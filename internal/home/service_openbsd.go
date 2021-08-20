//go:build openbsd
// +build openbsd

package home

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"text/template"

	"github.com/AdguardTeam/AdGuardHome/internal/aghos"
	"github.com/AdguardTeam/golibs/errors"
	"github.com/AdguardTeam/golibs/stringutil"
	"github.com/kardianos/service"
)

// OpenBSD Service Implementation
//
// The file contains OpenBSD implementations for service.System and
// service.Service.  It uses the default approach for RunCom-based service
// systems, e.g. rc.d script.
//
// TODO(e.burkov):  Perhaps, file a PR to github.com/kardianos/service.

const (
	// sysVersion is the version of local service.System interface
	// implementation.
	sysVersion = "openbsd-runcom"

	// errNoUserServiceRunCom is returned when the service uses some custom
	// path to script.
	errNoUserServiceRunCom errors.Error = "user services are not supported on " + sysVersion
)

func chooseSystem() {
	service.ChooseSystem(openbsdSystem{})
}

// type check
var _ service.System = openbsdSystem{}

// openbsdSystem is the service.System to be used on the OpenBSD.
type openbsdSystem struct{}

// String implements service.System interface for openbsdSystem.
func (openbsdSystem) String() string {
	return sysVersion
}

// Detect implements service.System interface for openbsdSystem.
func (openbsdSystem) Detect() (ok bool) {
	return true
}

// Interactive implements service.System interface for openbsdSystem.
func (openbsdSystem) Interactive() (ok bool) {
	return os.Getppid() != 1
}

// New implements service.System interface for openbsdSystem.
func (openbsdSystem) New(i service.Interface, c *service.Config) (s service.Service, err error) {
	return &openbsdRunComService{
		i:      i,
		Config: c,
	}, nil
}

// openbsdRunComService is the RunCom-based service.Service to be used on the
// OpenBSD.
type openbsdRunComService struct {
	i service.Interface
	// *service.Config is embedded here to avoid implementing all
	// service.Service interface's methods.
	*service.Config
}

// Platform implements service.Service interface for *openbsdRunComService.
func (*openbsdRunComService) Platform() (p string) {
	return `openbsd`
}

// String implements service.Service interface for *openbsdRunComService.
func (s *openbsdRunComService) String() string {
	return stringutil.Coalesce(s.DisplayName, s.Name)
}

// getBool returns the value of the given name from kv, assuming the value is a
// boolean.  If the value isn't found or is not of the type, the defaultValue is
// returned.
func getBool(kv service.KeyValue, name string, defaultValue bool) (val bool) {
	var ok bool
	if val, ok = kv[name].(bool); ok {
		return val
	}

	return defaultValue
}

// getString returns the value of the given name from kv, assuming the value is
// a string.  If the value isn't found or is not of the type, the defaultValue
// is returned.
func getString(kv service.KeyValue, name, defaultValue string) (val string) {
	var ok bool
	if val, ok = kv[name].(string); ok {
		return val
	}

	return defaultValue
}

// getFuncSingle returns the value of the given name from kv, assuming the value
// is a func().  If the value isn't found or is not of the type, the
// defaultValue is returned.
func getFuncSingle(kv service.KeyValue, name string, defaultValue func()) (val func()) {
	var ok bool
	if val, ok = kv[name].(func()); ok {
		return val
	}

	return defaultValue
}

const (
	// optionUserService is the UserService option name.
	optionUserService = "UserService"
	// optionUserServiceDefault is the UserService option default value.
	optionUserServiceDefault = false
)

// scriptPath returns the absolute path to the script.  It's commonly used to
// send commands to the service.
func (s *openbsdRunComService) scriptPath() (cp string, err error) {
	if getBool(s.Option, optionUserService, optionUserServiceDefault) {
		return "", errNoUserServiceRunCom
	}

	const scriptPathPref = "/etc/rc.d"

	return filepath.Join(scriptPathPref, s.Config.Name), nil
}

const (
	// optionRunComScript is the RunCom script option name.
	optionRunComScript = "RunComScript"
	// runComScript is the default RunCom script.
	runComScript = `#!/bin/sh
#
# $OpenBSD: {{ .SvcInfo }}

daemon="{{.Path}}"
daemon_flags={{ .Arguments | args }}

. /etc/rc.d/rc.subr

rc_bg=YES

rc_cmd $1
`
)

// template returns the script template to put into rc.d.
func (s *openbsdRunComService) template() (t *template.Template) {
	tf := map[string]interface{}{
		"args": func(sl []string) string {
			return `"` + strings.Join(sl, ` `) + `"`
		},
	}

	return template.Must(template.New("").Funcs(tf).Parse(getString(
		s.Option,
		optionRunComScript,
		runComScript,
	)))
}

// execPath returns the absolute path to the excutable to be run as a service.
func (s *openbsdRunComService) execPath() (path string, err error) {
	if c := s.Config; c != nil && len(c.Executable) != 0 {
		return filepath.Abs(c.Executable)
	}

	if path, err = os.Executable(); err != nil {
		return "", err
	}

	return filepath.Abs(path)
}

// Install implements service.Service interface for *openbsdRunComService.
func (s *openbsdRunComService) Install() (err error) {
	defer func() { errors.Annotate(err, "installing %s %s service: %w", sysVersion, s.Name) }()

	var scriptPath string
	if scriptPath, err = s.scriptPath(); err != nil {
		return err
	}

	if _, err = os.Stat(scriptPath); !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("script already exists at %s", scriptPath)
	}

	var execPath string
	if execPath, err = s.execPath(); err != nil {
		return err
	}

	t := s.template()
	f, err := os.Create(scriptPath)
	if err != nil {
		return fmt.Errorf("creating rc.d script file: %w", err)
	}
	defer f.Close()

	err = t.Execute(f, &struct {
		*service.Config
		Path    string
		SvcInfo string
	}{
		Config:  s.Config,
		Path:    execPath,
		SvcInfo: getString(s.Config.Option, "SvcInfo", s.String()),
	})
	if err != nil {
		return err
	}

	return errors.Annotate(
		os.Chmod(scriptPath, 0o755),
		"changing rc.d script file permissions: %w",
	)
}

// Uninstall implements service.Service interface for *openbsdRunComService.
func (s *openbsdRunComService) Uninstall() (err error) {
	defer func() { errors.Annotate(err, "uninstalling %s %s service: %w", sysVersion, s.Name) }()

	var scriptPath string
	if scriptPath, err = s.scriptPath(); err != nil {
		return err
	}

	return errors.Annotate(os.Remove(scriptPath), "removing rc.d script: %w")
}

// Logger implements service.Service interface for *openbsdRunComService.
func (s *openbsdRunComService) Logger(errs chan<- error) (l service.Logger, err error) {
	if service.ChosenSystem().Interactive() {
		return service.ConsoleLogger, nil
	}

	return s.SystemLogger(errs)
}

// SystemLogger implements service.Service interface for *openbsdRunComService.
func (s *openbsdRunComService) SystemLogger(errs chan<- error) (l service.Logger, err error) {
	return newSysLogger(s.Name, errs)
}

const optionRunWait = "RunWait"

// Run implements service.Service interface for *openbsdRunComService.
func (s *openbsdRunComService) Run() (err error) {
	if err = s.i.Start(s); err != nil {
		return err
	}

	getFuncSingle(s.Option, optionRunWait, func() {
		sigChan := make(chan os.Signal, 3)
		signal.Notify(sigChan, syscall.SIGTERM, os.Interrupt)
		<-sigChan
	})()

	return s.i.Stop(s)
}

func (s *openbsdRunComService) runCom(cmd string) (out string, err error) {
	var scriptPath string
	if scriptPath, err = s.scriptPath(); err != nil {
		return "", err
	}

	_, out, err = aghos.RunCommand(scriptPath, cmd)

	return out, err
}

// Status implements service.Service interface for *openbsdRunComService.
func (s *openbsdRunComService) Status() (status service.Status, err error) {
	defer func() { errors.Annotate(err, "getting %s %s service status: %w", sysVersion, s.Name) }()

	var out string
	if out, err = s.runCom("check"); err != nil {
		return service.StatusUnknown, err
	}

	switch name := s.Config.Name; out {
	case fmt.Sprintf("%s(ok)\n", name):
		return service.StatusRunning, nil
	case fmt.Sprintf("%s(failed)\n", name):
		return service.StatusStopped, nil
	default:
		return service.StatusUnknown, service.ErrNotInstalled
	}
}

// Start implements service.Service interface for *openbsdRunComService.
func (s *openbsdRunComService) Start() (err error) {
	_, err = s.runCom("start")

	return errors.Annotate(err, "starting %s %s service: %w", sysVersion, s.Name)
}

// Stop implements service.Service interface for *openbsdRunComService.
func (s *openbsdRunComService) Stop() (err error) {
	_, err = s.runCom("stop")

	return errors.Annotate(err, "stopping %s %s service: %w", sysVersion, s.Name)
}

// Restart implements service.Service interface for *openbsdRunComService.
func (s *openbsdRunComService) Restart() (err error) {
	if err = s.Stop(); err != nil {
		return err
	}

	return s.Start()
}
