package app

import (
	"fmt"
	"net/http"

	_ "expvar"
	_ "net/http/pprof"
)

// SyslogAgent manages starting the syslog agent service.
type SyslogAgent struct {
	cfg Config
}

// NewSyslogAgent intializes and returns a new SyslogAgent.
func NewSyslogAgent(cfg Config) *SyslogAgent {
	return &SyslogAgent{
		cfg: cfg,
	}
}

// Run starts all the sub-processes of the syslog agent. If blocking is
// true this method will block otherwise it will return immediately and run
// the syslog agent a goroutine.
func (s SyslogAgent) Run(blocking bool) {
	if blocking {
		s.run()
		return
	}

	go s.run()
}

func (s SyslogAgent) run() {
	http.ListenAndServe(fmt.Sprintf("127.0.0.1:%d", s.cfg.DebugPort), nil)
}
