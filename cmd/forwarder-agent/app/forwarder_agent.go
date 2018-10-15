package app

import (
	"fmt"
	"log"
	"sync"
	"time"

	"net/http"
	_ "net/http/pprof"

	gendiodes "code.cloudfoundry.org/go-diodes"
	egress "code.cloudfoundry.org/loggregator-agent/pkg/egress/v2"
	ingress "code.cloudfoundry.org/loggregator-agent/pkg/ingress/v2"

	"code.cloudfoundry.org/loggregator-agent/pkg/diodes"
	"code.cloudfoundry.org/loggregator-agent/pkg/ingress/cups"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

// ForwarderAgent manages starting the forwarder agent service.
type ForwarderAgent struct {
	debugPort        uint16
	pollingInterval  time.Duration
	drainCountMetric func(float64)
	bf               bindingFetcher
	buffer           *diodes.ManyToOneEnvelopeV2
	loggregatorAddr  string

	mu            sync.Mutex
	ingressServer *ingress.Server
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
	loggregatorAddr string,
	m metrics,
	bf bindingFetcher,
	pi time.Duration,
) *ForwarderAgent {
	buffer := diodes.NewManyToOneEnvelopeV2(10000, gendiodes.AlertFunc(func(missed int) {
		// metric-documentation-v2: (loggregator.metron.dropped) Number of v2 envelopes
		// dropped from the agent ingress diode
		// droppedMetric.Increment(uint64(missed))

		log.Printf("Dropped %d v2 envelopes", missed)
	}))

	return &ForwarderAgent{
		debugPort:        debugPort,
		pollingInterval:  pi,
		drainCountMetric: m.NewGauge("drainCount"),
		bf:               bf,
		buffer:           buffer,
		loggregatorAddr:  loggregatorAddr,
	}
}

func (s *ForwarderAgent) IngressAddr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ingressServer == nil {
		return ""
	}

	return s.ingressServer.Addr()
}

// Run starts all the sub-processes of the forwarder agent. If blocking is
// true this method will block otherwise it will return immediately and run
// the forwarder agent a goroutine.
func (s *ForwarderAgent) Run(blocking bool) {
	if blocking {
		s.run()
		return
	}

	go s.run()
}

func (s *ForwarderAgent) run() {
	go s.reportBindings()
	go s.startIngressServer()
	go s.startEgress()

	http.ListenAndServe(fmt.Sprintf("127.0.0.1:%d", s.debugPort), nil)
}

func (s *ForwarderAgent) startIngressServer() {
	agentAddress := "127.0.0.1:0"
	// agentAddress := fmt.Sprintf("127.0.0.1:%d", a.config.GRPC.Port)
	log.Printf("agent v2 API started on addr %s", agentAddress)

	rx := ingress.NewReceiver(s.buffer, nil, nil)
	kp := keepalive.EnforcementPolicy{
		MinTime:             10 * time.Second,
		PermitWithoutStream: true,
	}
	s.mu.Lock()
	s.ingressServer = ingress.NewServer(
		agentAddress,
		rx,
		// grpc.Creds(a.serverCreds),
		grpc.KeepaliveEnforcementPolicy(kp),
	)
	s.mu.Unlock()
	s.ingressServer.Start()
}

func (s *ForwarderAgent) startEgress() {
	lf := egress.NewEnvelopeForwarder(
		s.loggregatorAddr,
		s.buffer,
		grpc.WithInsecure(),
	)
	lf.Run(true)
}

func (s *ForwarderAgent) reportBindings() {
	t := time.NewTicker(s.pollingInterval)

	for range t.C {
		bindings, _ := s.bf.FetchBindings()
		s.drainCountMetric(float64(len(bindings)))
	}
}
