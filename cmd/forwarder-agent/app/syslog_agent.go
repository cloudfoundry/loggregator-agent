package app

import (
	"fmt"
	"net/http"

	_ "expvar"
	_ "net/http/pprof"
)

// ForwarderAgent manages starting the forwarder agent service.
type ForwarderAgent struct {
	cfg Config
}

// NewForwarderAgent intializes and returns a new forwarder agent.
func NewForwarderAgent(cfg Config) *ForwarderAgent {
	return &ForwarderAgent{
		cfg: cfg,
	}
}

// Run starts all the sub-processes of the forwarder agent. If blocking is
// true this method will block otherwise it will return immediately and run
// the forwarder agent a goroutine.
func (s ForwarderAgent) Run(blocking bool) {
	if blocking {
		s.run()
		return
	}

	go s.run()
}

func (s ForwarderAgent) run() {
	http.ListenAndServe(fmt.Sprintf("127.0.0.1:%d", s.cfg.DebugPort), nil)
}
