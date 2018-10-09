package app

import (
	"fmt"
	"net/http"

	_ "expvar"
	_ "net/http/pprof"
)

type SyslogAgent struct {
	cfg Config
}

func NewSyslogAgent(cfg Config) *SyslogAgent {
	return &SyslogAgent{
		cfg: cfg,
	}
}

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
