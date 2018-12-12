package binding_test

import (
	"context"
	"errors"
	"log"
	"sync/atomic"
	"time"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator-agent/pkg/binding"
	"code.cloudfoundry.org/loggregator-agent/pkg/egress"
	"code.cloudfoundry.org/loggregator-agent/pkg/egress/syslog"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Manager", func() {

	var (
		bf *stubBindingFetcher
		sm *spyMetrics
		c  *spyConnector
	)

	BeforeEach(func() {
		bf = newStubBindingFetcher()
		sm = newSpyMetrics()
		c = newSpyConnector()
	})

	It("reports the number of binding that come from the fetcher", func() {
		bf.bindings <- []syslog.Binding{
			{"app-1", "host-1", "syslog://drain.url.com"},
			{"app-2", "host-2", "syslog://drain.url.com"},
			{"app-3", "host-3", "syslog://drain.url.com"},
		}

		m := binding.NewManager(
			bf,
			c,
			sm,
			100*time.Millisecond,
			log.New(GinkgoWriter, "", 0),
		)
		go m.Run()

		var mv float64
		Eventually(sm.metricValues).Should(Receive(&mv))
		Expect(mv).To(BeNumerically("==", 3))
	})

	It("polls for updates from the binding fetcher", func() {
		bf.bindings <- []syslog.Binding{
			{"app-1", "host-1", "syslog://drain.url.com"},
			{"app-3", "host-3", "syslog://drain.url.com"},
		}
		bf.bindings <- []syslog.Binding{
			{"app-1", "host-1", "syslog://drain.url.com"},
			{"app-2", "host-2", "syslog://drain.url.com"},
			{"app-3", "host-3", "syslog://drain.url.com"},
		}

		go func(bindings chan []syslog.Binding) {
			for {
				bindings <- []syslog.Binding{
					{"app-2", "host-2", "syslog://drain.url.com"},
					{"app-3", "host-3", "syslog://drain.url.com"},
				}
			}
		}(bf.bindings)

		m := binding.NewManager(
			bf,
			c,
			sm,
			100*time.Millisecond,
			log.New(GinkgoWriter, "", 0),
		)
		go m.Run()

		Eventually(sm.metricValues).Should(HaveLen(2))
		Expect(<-sm.metricValues).To(BeNumerically("==", 2))
		Expect(<-sm.metricValues).To(BeNumerically("==", 3))

		Eventually(func() []egress.Writer {
			return m.GetDrains("app-2")
		}).Should(HaveLen(1))
		Eventually(func() []egress.Writer {
			return m.GetDrains("app-3")
		}).Should(HaveLen(1))
		Expect(c.ConnectionCount()).Should(BeNumerically("==", 3))

		// Also remove old drains when updating
		Eventually(func() []egress.Writer {
			return m.GetDrains("app-1")
		}).Should(HaveLen(0))

		closedBdg := syslog.Binding{"app-1", "host-1", "syslog://drain.url.com"}
		closedCtx := c.bindingContextMap[closedBdg]
		Expect(closedCtx.Err()).To(Equal(errors.New("context canceled")))
	})

	It("returns drains for a sourceID", func() {
		bf.bindings <- []syslog.Binding{
			{"app-1", "host-1", "syslog://drain.url.com"},
			{"app-2", "host-2", "syslog://drain.url.com"},
			{"app-3", "host-3", "syslog://drain.url.com"},
		}

		m := binding.NewManager(
			bf,
			c,
			sm,
			10*time.Second,
			log.New(GinkgoWriter, "", 0),
		)
		go m.Run()

		var appDrains []egress.Writer
		Eventually(func() int {
			appDrains = m.GetDrains("app-1")
			return len(appDrains)
		}).Should(Equal(1))

		e := &loggregator_v2.Envelope{
			Timestamp: time.Now().UnixNano(),
			SourceId:  "app-1",
			Message: &loggregator_v2.Envelope_Log{
				Log: &loggregator_v2.Log{
					Payload: []byte("hello"),
				},
			},
		}

		appDrains[0].Write(e)

		var env *loggregator_v2.Envelope
		Eventually(appDrains[0].(*spyDrain).envelopes).Should(Receive(&env))
		Expect(env).To(Equal(e))
	})

	It("maintains current state on error", func() {
		bf.bindings <- []syslog.Binding{
			{"app-1", "host-1", "syslog://drain.url.com"},
		}

		m := binding.NewManager(
			bf,
			c,
			sm,
			10*time.Millisecond,
			log.New(GinkgoWriter, "", 0),
		)
		go m.Run()

		Eventually(func() int {
			return len(m.GetDrains("app-1"))
		}).Should(Equal(1))

		bf.errors <- errors.New("boom")

		Consistently(func() int {
			return len(m.GetDrains("app-1"))
		}).Should(Equal(1))
	})
})

type spyDrain struct {
	// mu sync.Mutex
	envelopes chan *loggregator_v2.Envelope
}

func newSpyDrain() *spyDrain {
	return &spyDrain{
		envelopes: make(chan *loggregator_v2.Envelope, 100),
	}
}

func (s *spyDrain) Write(e *loggregator_v2.Envelope) error {
	s.envelopes <- e
	return nil
}

type spyConnector struct {
	connectionCount   int64
	bindingContextMap map[syslog.Binding]context.Context
}

func newSpyConnector() *spyConnector {
	return &spyConnector{
		bindingContextMap: make(map[syslog.Binding]context.Context),
	}
}

func (c *spyConnector) ConnectionCount() int64 {
	return atomic.LoadInt64(&c.connectionCount)
}

func (c *spyConnector) Connect(ctx context.Context, b syslog.Binding) (egress.Writer, error) {
	c.bindingContextMap[b] = ctx
	atomic.AddInt64(&c.connectionCount, 1)
	return newSpyDrain(), nil
}

type stubBindingFetcher struct {
	bindings chan []syslog.Binding
	errors   chan error
}

func newStubBindingFetcher() *stubBindingFetcher {
	return &stubBindingFetcher{
		bindings: make(chan []syslog.Binding, 100),
		errors:   make(chan error, 100),
	}
}

func (s *stubBindingFetcher) FetchBindings() ([]syslog.Binding, error) {
	select {
	case b := <-s.bindings:
		return b, nil
	case err := <-s.errors:
		return nil, err
	}
}

type spyMetrics struct {
	name         string
	metricValues chan float64
}

func newSpyMetrics() *spyMetrics {
	return &spyMetrics{
		metricValues: make(chan float64, 100),
	}
}

func (sm *spyMetrics) NewGauge(name string) func(float64) {
	sm.name = name
	return func(val float64) {
		sm.metricValues <- val
	}
}

func (sm *spyMetrics) NewCounter(name string) func(uint64) {
	sm.name = name
	return func(val uint64) {
		sm.metricValues <- float64(val)
	}
}
