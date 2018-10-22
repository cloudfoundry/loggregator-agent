package binding

import (
	"context"
	"log"
	"sync"
	"time"

	"code.cloudfoundry.org/loggregator-agent/pkg/egress/syslog"
)

type Fetcher interface {
	FetchBindings() ([]syslog.Binding, error)
}

type Metrics interface {
	NewGauge(string) func(float64)
	NewCounter(name string) func(uint64)
}

type Connector interface {
	Connect(context.Context, syslog.Binding) (syslog.Writer, error)
}

type drainHolder struct {
	cancel      func()
	drainWriter syslog.Writer
}

type Manager struct {
	sync.Mutex
	bf               Fetcher
	drainCountMetric func(float64)
	pollingInterval  time.Duration
	sourceDrainMap   map[string]map[syslog.Binding]drainHolder
	c                Connector
	log              *log.Logger
}

func NewManager(bf Fetcher, connector Connector, m Metrics, pi time.Duration, log *log.Logger) *Manager {
	return &Manager{
		bf:               bf,
		drainCountMetric: m.NewGauge("DrainCount"),
		pollingInterval:  pi,
		sourceDrainMap:   make(map[string]map[syslog.Binding]drainHolder),
		c:                connector,
		log:              log,
	}
}

func (m *Manager) Run() {
	t := time.NewTicker(m.pollingInterval)

	bindings, _ := m.bf.FetchBindings()
	m.drainCountMetric(float64(len(bindings)))
	m.updateDrains(bindings)
	for range t.C {
		bindings, _ := m.bf.FetchBindings()
		m.drainCountMetric(float64(len(bindings)))
		m.updateDrains(bindings)
	}
}

func (m *Manager) updateDrains(bindings []syslog.Binding) {
	m.Lock()
	defer m.Unlock()

	newBindings := make(map[syslog.Binding]bool)

	for _, b := range bindings {
		newBindings[b] = true

		_, ok := m.sourceDrainMap[b.AppId][b]
		if ok {
			continue
		}

		ctx, cancel := context.WithCancel(context.Background())
		writer, err := m.c.Connect(ctx, b)
		if err != nil {
			m.log.Printf("Failed to create binding: %s", err)
		}

		_, ok = m.sourceDrainMap[b.AppId]
		if !ok {
			m.sourceDrainMap[b.AppId] = make(map[syslog.Binding]drainHolder)
		}
		m.sourceDrainMap[b.AppId][b] = drainHolder{
			cancel:      cancel,
			drainWriter: writer,
		}
	}

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
		}
	}
}

func (m *Manager) GetDrains(sourceID string) []syslog.Writer {
	m.Lock()
	defer m.Unlock()

	existing := m.sourceDrainMap[sourceID]
	drains := make([]syslog.Writer, 0, len(existing))
	for _, dh := range existing {
		drains = append(drains, dh.drainWriter)
	}

	return drains
}
