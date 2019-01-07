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
	mu              sync.Mutex
	bf              Fetcher
	bfLimit         int
	pollingInterval time.Duration
	connector       Connector
	log             *log.Logger

	drainCountMetric       func(float64)
	activeDrainCountMetric func(float64)
	activeDrainCount       int64

	sourceDrainMap map[string]map[syslog.Binding]drainHolder
}

func NewManager(
	bf Fetcher,
	connector Connector,
	m Metrics,
	pi time.Duration,
	log *log.Logger,
) *Manager {
	return &Manager{
		bf:                     bf,
		bfLimit:                bf.DrainLimit(),
		pollingInterval:        pi,
		connector:              connector,
		drainCountMetric:       m.NewGauge("DrainCount"),
		activeDrainCountMetric: m.NewGauge("ActiveDrainCount"),
		sourceDrainMap:         make(map[string]map[syslog.Binding]drainHolder),
		log:                    log,
	}
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

		ctx, cancel := context.WithCancel(context.Background())
		m.sourceDrainMap[b.AppId][b] = drainHolder{
			ctx:         ctx,
			cancel:      cancel,
			drainWriter: nil,
		}
	}

	// Delete all bindings that are not in updated list of bindings.
	// TODO: this is not optimal, consider lazily storing bindings
	for appID, bindingWriterMap := range m.sourceDrainMap {
		for b, _ := range bindingWriterMap {
			if newBindings[b] {
				continue
			}

			bindingWriterMap[b].cancel()
			delete(bindingWriterMap, b)
			if len(bindingWriterMap) == 0 {
				// Prevent memory leak
				delete(m.sourceDrainMap, appID)
			}

			m.activeDrainCount--
			m.activeDrainCountMetric(float64(m.activeDrainCount))
		}
	}
}

type drainHolder struct {
	ctx         context.Context
	cancel      func()
	drainWriter egress.Writer
}
