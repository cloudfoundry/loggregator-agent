package binding

import (
	"context"
	"log"
	"math/rand"
	"sync"
	"time"

	"code.cloudfoundry.org/loggregator-agent/pkg/egress"
	"code.cloudfoundry.org/loggregator-agent/pkg/egress/syslog"
)

type Fetcher interface {
	FetchBindings() ([]syslog.Binding, error)
	DrainLimit() int
}

type Metrics interface {
	NewGauge(string) func(float64)
	NewCounter(name string) func(uint64)
}

type Connector interface {
	Connect(context.Context, syslog.Binding) (egress.Writer, error)
}

type Manager struct {
	mu        sync.Mutex
	bf        Fetcher
	bfLimit   int
	connector Connector
	log       *log.Logger

	pollingInterval time.Duration
	idleTimeout     time.Duration

	drainCountMetric       func(float64)
	activeDrainCountMetric func(float64)
	activeDrainCount       int64

	sourceDrainMap    sync.Map
	sourceAccessTimes sync.Map
}

func NewManager(
	bf Fetcher,
	c Connector,
	m Metrics,
	pollingInterval time.Duration,
	idleTimeout time.Duration,
	log *log.Logger,
) *Manager {
	manager := &Manager{
		bf:                     bf,
		bfLimit:                bf.DrainLimit(),
		pollingInterval:        pollingInterval,
		idleTimeout:            idleTimeout,
		connector:              c,
		drainCountMetric:       m.NewGauge("DrainCount"),
		activeDrainCountMetric: m.NewGauge("ActiveDrainCount"),
		log: log,
	}

	go manager.idleCleanupLoop()

	return manager
}

func (m *Manager) Run() {
	bindings, _ := m.bf.FetchBindings()
	m.drainCountMetric(float64(len(bindings)))
	m.updateDrains(bindings)

	offset := rand.Int63n(m.pollingInterval.Nanoseconds())
	t := time.NewTicker(m.pollingInterval + time.Duration(offset))
	for range t.C {
		bindings, err := m.bf.FetchBindings()
		if err != nil {
			m.log.Printf("failed to fetch bindings: %s", err)
			continue
		}

		m.drainCountMetric(float64(len(bindings)))
		m.updateDrains(bindings)
		m.log.Printf("updated drain bindings with %d bindings", len(bindings))
	}
}

func (m *Manager) GetDrains(sourceID string) []egress.Writer {
	m.sourceAccessTimes.Store(sourceID, time.Now())
	drains := make([]egress.Writer, 0, m.bfLimit)
	a, ok := m.sourceDrainMap.Load(sourceID)
	if !ok {
		return drains
	}

	appBindings := a.(*sync.Map)
	appBindings.Range(func(b, d interface{}) bool {
		binding := b.(syslog.Binding)
		dh := d.(drainHolder)

		// Create drain writer if one does not already exist
		if dh.drainWriter == nil {
			writer, err := m.connector.Connect(dh.ctx, binding)
			if err != nil {
				m.log.Printf("failed to create binding: %s", err)
			}

			dh.drainWriter = writer
			appBindings.Store(binding, dh)

			m.activeDrainCount++
			m.activeDrainCountMetric(float64(m.activeDrainCount))
		}

		drains = append(drains, dh.drainWriter)

		return true
	})

	return drains
}

func (m *Manager) updateDrains(bindings []syslog.Binding) {
	newBindings := make(map[syslog.Binding]bool)

	for _, b := range bindings {
		newBindings[b] = true

		ab, _ := m.sourceDrainMap.LoadOrStore(b.AppId, &sync.Map{})
		appBindings := ab.(*sync.Map)
		appBindings.LoadOrStore(b, newDrainHolder())
	}

	// Delete all bindings that are not in updated list of bindings.
	// TODO: this is not optimal, consider lazily storing bindings
	m.sourceDrainMap.Range(func(_, a interface{}) bool {
		appBindings := a.(*sync.Map)
		appBindings.Range(func(b, _ interface{}) bool {
			binding := b.(syslog.Binding)
			if newBindings[binding] {
				return true
			}

			m.removeDrain(appBindings, binding)
			return true
		})
		return true
	})
}

func (m *Manager) removeDrain(
	bindingWriterMap *sync.Map,
	b syslog.Binding,
) {
	w, _ := bindingWriterMap.Load(b)
	writer := w.(drainHolder)

	var active bool
	if writer.drainWriter != nil {
		active = true
	}

	writer.cancel()
	bindingWriterMap.Delete(b)
	// if len(bindingWriterMap) == 0 {
	// 	// Prevent memory leak
	// 	m.SourceDrainMap.Delete(b.AppId)
	// }

	if active {
		m.activeDrainCount--
		m.activeDrainCountMetric(float64(m.activeDrainCount))
	}
}

func (m *Manager) idleCleanupLoop() {
	t := time.NewTicker(m.idleTimeout)
	for range t.C {
		m.idleCleanup()
	}
}

func (m *Manager) idleCleanup() {
	m.log.Println("starting cleanup of idle drains")
	currentTime := time.Now()
	m.sourceAccessTimes.Range(func(s, t interface{}) bool {
		sID := s.(string)
		ts := t.(time.Time)

		if ts.Before(currentTime.Add(-m.idleTimeout)) {
			m.log.Println("removing idle drain for source", sID)

			a, _ := m.sourceDrainMap.Load(sID)
			appBindings := a.(*sync.Map)
			appBindings.Range(func(b, d interface{}) bool {
				binding := b.(syslog.Binding)
				dh := d.(drainHolder)

				dh.cancel()

				appBindings.Store(binding, newDrainHolder())

				m.activeDrainCount--
				m.activeDrainCountMetric(float64(m.activeDrainCount))

				return true
			})

			m.sourceAccessTimes.Delete(sID)
		}
		return true
	})
}

type drainHolder struct {
	ctx         context.Context
	cancel      func()
	drainWriter egress.Writer
}

func newDrainHolder() drainHolder {
	ctx, cancel := context.WithCancel(context.Background())
	return drainHolder{
		ctx:         ctx,
		cancel:      cancel,
		drainWriter: nil,
	}
}
