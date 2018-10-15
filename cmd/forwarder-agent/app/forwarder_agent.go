package app

import (
	"fmt"
	"time"

	"net/http"
	_ "net/http/pprof"

	"code.cloudfoundry.org/loggregator-agent/pkg/ingress/cups"
)

// ForwarderAgent manages starting the forwarder agent service.
type ForwarderAgent struct {
	debugPort        uint16
	pollingInterval  time.Duration
	drainCountMetric func(float64)
	bf               bindingFetcher
}

type metrics interface {
	NewGauge(string) func(float64)
}

type bindingFetcher interface {
	FetchBindings() ([]cups.Binding, error)
}

// NewForwarderAgent intializes and returns a new forwarder agent.
func NewForwarderAgent(
	debugPort uint16,
	m metrics,
	bf bindingFetcher,
	pi time.Duration,
) *ForwarderAgent {
	return &ForwarderAgent{
		debugPort:        debugPort,
		pollingInterval:  pi,
		drainCountMetric: m.NewGauge("drainCount"),
		bf:               bf,
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
	go s.reportBindings()

	http.ListenAndServe(fmt.Sprintf("127.0.0.1:%d", s.debugPort), nil)
}

func (s ForwarderAgent) reportBindings() {
	t := time.NewTicker(s.pollingInterval)

	for range t.C {
		bindings, _ := s.bf.FetchBindings()
		s.drainCountMetric(float64(len(bindings)))
	}
}
