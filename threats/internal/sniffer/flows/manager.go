package flows

import (
	"sync"
	"threats/internal/classifier/model"
	"time"
)

type FlowsManager struct {
	flows    map[FlowKey]*Flow
	mu       sync.Mutex
	timeout  time.Duration
	exporter func(*model.TrafficParameters)

	done chan struct{}
}

func NewFlowsManager(exporter func(*model.TrafficParameters)) *FlowsManager {
	mngr := &FlowsManager{
		flows:    make(map[FlowKey]*Flow),
		exporter: exporter,
		timeout:  1 * time.Second,
		done:     make(chan struct{}),
	}

	go mngr.startCleanupLoop()

	return mngr
}

func (m *FlowsManager) AddEvent(evt FlowEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key, err := ExtractKeyFromPacket(evt.Packet)

	if err != nil {
		return
	}

	flow, exists := m.flows[*key]
	if !exists {
		flow = NewFlow(*key)
		m.flows[*key] = flow
	}

	flow.AddPacket(evt)
}

func (m *FlowsManager) startCleanupLoop() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.cleanExpiredFlows()
		case <-m.done:
			return
		}
	}
}

func (m *FlowsManager) cleanExpiredFlows() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for key, flow := range m.flows {
		if flow.IsExpired(m.timeout) {

			params := flow.Export()
			m.exporter(&params)
			delete(m.flows, key)
		}
	}
}

func (m *FlowsManager) Stop() {
	close(m.done)
}
