package state

import (
	"context"
	"sync"
	"time"

	"github.com/stratastor/logger"
)

// ServiceState represents the state of a service
type ServiceState string

const (
	StateUnknown    ServiceState = "unknown"
	StateStarting   ServiceState = "starting"
	StateRunning    ServiceState = "running"
	StateStopping   ServiceState = "stopping"
	StateStopped    ServiceState = "stopped"
	StateRestarting ServiceState = "restarting"
	StateFailed     ServiceState = "failed"
)

// StateChangeEvent represents a service state change event
type StateChangeEvent struct {
	ServiceName string
	PrevState   ServiceState
	NewState    ServiceState
	Timestamp   time.Time
	Details     map[string]interface{}
}

// StateChangeCallback is called when a service state changes
type StateChangeCallback func(ctx context.Context, event StateChangeEvent) error

// ServiceStateManager manages service states
type ServiceStateManager struct {
	logger     logger.Logger
	states     map[string]ServiceState
	callbacks  []StateChangeCallback
	mu         sync.RWMutex
	reconciler *StateReconciler
}

// NewServiceStateManager creates a new service state manager
func NewServiceStateManager(logger logger.Logger) *ServiceStateManager {
	manager := &ServiceStateManager{
		logger:    logger,
		states:    make(map[string]ServiceState),
		callbacks: []StateChangeCallback{},
	}

	// Create reconciler
	manager.reconciler = NewStateReconciler(logger, manager)

	return manager
}

// RegisterCallback registers a state change callback
func (m *ServiceStateManager) RegisterCallback(callback StateChangeCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callbacks = append(m.callbacks, callback)
}

// GetState returns the current state of a service
func (m *ServiceStateManager) GetState(serviceName string) ServiceState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state, ok := m.states[serviceName]
	if !ok {
		return StateUnknown
	}
	return state
}

// SetState sets the state of a service
func (m *ServiceStateManager) SetState(
	ctx context.Context,
	serviceName string,
	newState ServiceState,
	details map[string]interface{},
) error {
	m.mu.Lock()

	prevState, ok := m.states[serviceName]
	if !ok {
		prevState = StateUnknown
	}
	m.states[serviceName] = newState

	m.mu.Unlock()

	// Create event
	event := StateChangeEvent{
		ServiceName: serviceName,
		PrevState:   prevState,
		NewState:    newState,
		Timestamp:   time.Now(),
		Details:     details,
	}

	// Notify callbacks
	for _, callback := range m.callbacks {
		if err := callback(ctx, event); err != nil {
			m.logger.Warn("Failed to notify state change",
				"service", serviceName,
				"prevState", prevState,
				"newState", newState,
				"err", err,
			)
		}
	}

	m.logger.Info("Service state changed",
		"service", serviceName,
		"prevState", prevState,
		"newState", newState,
	)

	return nil
}

// StartReconciler starts the state reconciler
func (m *ServiceStateManager) StartReconciler(ctx context.Context) {
	m.reconciler.Start(ctx)
}

// StopReconciler stops the state reconciler
func (m *ServiceStateManager) StopReconciler() {
	m.reconciler.Stop()
}

// StateReconciler periodically reconciles service states
type StateReconciler struct {
	logger       logger.Logger
	stateManager *ServiceStateManager
	interval     time.Duration
	stopCh       chan struct{}
	services     map[string]func(context.Context) (ServiceState, error)
}

// NewStateReconciler creates a new state reconciler
func NewStateReconciler(logger logger.Logger, stateManager *ServiceStateManager) *StateReconciler {
	return &StateReconciler{
		logger:       logger,
		stateManager: stateManager,
		interval:     30 * time.Second, // Default interval
		stopCh:       make(chan struct{}),
		services:     make(map[string]func(context.Context) (ServiceState, error)),
	}
}

// RegisterService registers a service for state reconciliation
func (r *StateReconciler) RegisterService(
	serviceName string,
	stateProvider func(context.Context) (ServiceState, error),
) {
	r.services[serviceName] = stateProvider
}

// SetInterval sets the reconciliation interval
func (r *StateReconciler) SetInterval(interval time.Duration) {
	r.interval = interval
}

// Start starts the reconciliation loop
func (r *StateReconciler) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(r.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				r.reconcile(ctx)
			case <-r.stopCh:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Stop stops the reconciliation loop
func (r *StateReconciler) Stop() {
	close(r.stopCh)
}

// reconcile reconciles service states
func (r *StateReconciler) reconcile(ctx context.Context) {
	for serviceName, stateProvider := range r.services {
		currentState, err := stateProvider(ctx)
		if err != nil {
			r.logger.Warn("Failed to get service state",
				"service", serviceName,
				"err", err,
			)
			continue
		}

		// Only update if state has changed
		if r.stateManager.GetState(serviceName) != currentState {
			r.stateManager.SetState(ctx, serviceName, currentState, nil)
		}
	}
}
