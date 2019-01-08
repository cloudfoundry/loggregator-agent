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

	sourceDrainMap    map[string]map[syslog.Binding]drainHolder
	sourceAccessTimes map[string]time.Time
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
		sourceDrainMap:         make(map[string]map[syslog.Binding]drainHolder),
		sourceAccessTimes:      make(map[string]time.Time),
		log:                    log,
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
	}
}

func (m *Manager) GetDrains(sourceID string) []egress.Writer {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.sourceAccessTimes[sourceID] = time.Now()
	drains := make([]egress.Writer, 0, m.bfLimit)
	for binding, dh := range m.sourceDrainMap[sourceID] {
		// Create drain writer if one does not already exist
		if dh.drainWriter == nil {
			writer, err := m.connector.Connect(dh.ctx, binding)
			if err != nil {
				m.log.Printf("Failed to create binding: %s", err)
			}

			dh.drainWriter = writer
			m.sourceDrainMap[sourceID][binding] = dh

			m.activeDrainCount++
			m.activeDrainCountMetric(float64(m.activeDrainCount))
		}

		drains = append(drains, dh.drainWriter)
	}

	return drains
}

func (m *Manager) updateDrains(bindings []syslog.Binding) {
	m.mu.Lock()
	defer m.mu.Unlock()

	newBindings := make(map[syslog.Binding]bool)

	for _, b := range bindings {
		newBindings[b] = true

		_, ok := m.sourceDrainMap[b.AppId][b]
		if ok {
			continue
		}

		_, ok = m.sourceDrainMap[b.AppId]
		if !ok {
			m.sourceDrainMap[b.AppId] = make(map[syslog.Binding]drainHolder)
		}

		m.sourceDrainMap[b.AppId][b] = newDrainHolder()
	}

	// Delete all bindings that are not in updated list of bindings.
	// TODO: this is not optimal, consider lazily storing bindings
	for _, bindingWriterMap := range m.sourceDrainMap {
		for b := range bindingWriterMap {
			if newBindings[b] {
				continue
			}

			m.removeDrain(bindingWriterMap, b)
		}
	}
}

func (m *Manager) removeDrain(
	bindingWriterMap map[syslog.Binding]drainHolder,
	b syslog.Binding,
) {
	var active bool
	if bindingWriterMap[b].drainWriter != nil {
		active = true
	}

	bindingWriterMap[b].cancel()
	delete(bindingWriterMap, b)
	if len(bindingWriterMap) == 0 {
		// Prevent memory leak
		delete(m.sourceDrainMap, b.AppId)
	}

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
	m.mu.Lock()
	defer m.mu.Unlock()

	m.log.Println("starting cleanup of idle drains")
	currentTime := time.Now()
	for sID, ts := range m.sourceAccessTimes {
		if ts.Before(currentTime.Add(-m.idleTimeout)) {
			m.log.Println("removing idle drain for source", sID)
			for b, dh := range m.sourceDrainMap[sID] {
				dh.cancel()

				m.sourceDrainMap[sID][b] = newDrainHolder()

				m.activeDrainCount--
				m.activeDrainCountMetric(float64(m.activeDrainCount))
			}

			delete(m.sourceAccessTimes, sID)
		}
	}
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
