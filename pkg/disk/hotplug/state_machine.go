// Copyright 2025 Raamsri Kumar <raam@tinkershack.in>
// Copyright 2025 The StrataSTOR Authors and Contributors
// SPDX-License-Identifier: Apache-2.0

package hotplug

import (
	"fmt"
	"time"

	"github.com/stratastor/logger"
	"github.com/stratastor/rodent/pkg/disk/types"
	"github.com/stratastor/rodent/pkg/errors"
)

// StateMachine manages disk state transitions for hotplug events.
//
// It validates that state transitions are valid according to the disk lifecycle:
//
//	UNKNOWN → DISCOVERED → VALIDATING → AVAILABLE → ONLINE
//	                           ↓            ↓          ↓
//	                       QUARANTINED  DEGRADED  FAULTED
//	                           ↓            ↓          ↓
//	                       REMOVING ← ← ← ← ← ← ← ← ←
//	                           ↓
//	                       OFFLINE → RETIRED
//
// The state machine prevents invalid transitions (e.g., AVAILABLE → RETIRED)
// and ensures the system maintains valid state throughout hotplug operations.
//
// All state changes triggered by hotplug events must pass through CanTransition()
// before being applied to the device state.
type StateMachine struct {
	logger logger.Logger

	// Valid state transitions
	transitions map[types.DiskState][]types.DiskState
}

// NewStateMachine creates a new state machine
func NewStateMachine(l logger.Logger) *StateMachine {
	sm := &StateMachine{
		logger:      l,
		transitions: make(map[types.DiskState][]types.DiskState),
	}

	// Define valid state transitions
	sm.defineTransitions()

	return sm
}

// defineTransitions defines all valid state transitions
func (sm *StateMachine) defineTransitions() {
	// From UNKNOWN state
	sm.transitions[types.DiskStateUnknown] = []types.DiskState{
		types.DiskStateDiscovered,
		types.DiskStateOffline,
	}

	// From DISCOVERED state
	sm.transitions[types.DiskStateDiscovered] = []types.DiskState{
		types.DiskStateValidating,
		types.DiskStateOffline,
		types.DiskStateQuarantined,
	}

	// From VALIDATING state
	sm.transitions[types.DiskStateValidating] = []types.DiskState{
		types.DiskStateAvailable,
		types.DiskStateDegraded,
		types.DiskStateFaulted,
		types.DiskStateQuarantined,
		types.DiskStateOffline,
	}

	// From AVAILABLE state
	sm.transitions[types.DiskStateAvailable] = []types.DiskState{
		types.DiskStateDegraded,
		types.DiskStateFaulted,
		types.DiskStateQuarantined,
		types.DiskStateOffline,
		types.DiskStateOnline,
	}

	// From ONLINE state
	sm.transitions[types.DiskStateOnline] = []types.DiskState{
		types.DiskStateAvailable,
		types.DiskStateDegraded,
		types.DiskStateFaulted,
		types.DiskStateQuarantined,
		types.DiskStateRemoving,
	}

	// From DEGRADED state
	sm.transitions[types.DiskStateDegraded] = []types.DiskState{
		types.DiskStateAvailable,
		types.DiskStateFaulted,
		types.DiskStateQuarantined,
		types.DiskStateOffline,
	}

	// From FAULTED state
	sm.transitions[types.DiskStateFaulted] = []types.DiskState{
		types.DiskStateQuarantined,
		types.DiskStateOffline,
		types.DiskStateRetired,
	}

	// From QUARANTINED state
	sm.transitions[types.DiskStateQuarantined] = []types.DiskState{
		types.DiskStateAvailable,
		types.DiskStateOffline,
		types.DiskStateRetired,
	}

	// From REMOVING state
	sm.transitions[types.DiskStateRemoving] = []types.DiskState{
		types.DiskStateOffline,
		types.DiskStateRetired,
	}

	// From OFFLINE state - can come back online
	sm.transitions[types.DiskStateOffline] = []types.DiskState{
		types.DiskStateDiscovered,
		types.DiskStateRetired,
	}

	// From RETIRED state - terminal state, no transitions
	sm.transitions[types.DiskStateRetired] = []types.DiskState{}

	// From UNAUTHORIZED state
	sm.transitions[types.DiskStateUnauthorized] = []types.DiskState{
		types.DiskStateDiscovered,
		types.DiskStateOffline,
	}
}

// CanTransition checks if a transition from oldState to newState is valid
func (sm *StateMachine) CanTransition(oldState, newState types.DiskState) bool {
	validNext, exists := sm.transitions[oldState]
	if !exists {
		return false
	}

	for _, state := range validNext {
		if state == newState {
			return true
		}
	}

	return false
}

// Transition attempts to transition a device to a new state
func (sm *StateMachine) Transition(
	deviceID string,
	oldState types.DiskState,
	newState types.DiskState,
	reason string,
	event *UdevEvent,
) (*DeviceTransition, error) {
	// Check if transition is valid
	if !sm.CanTransition(oldState, newState) {
		return nil, errors.New(errors.OperationFailed,
			fmt.Sprintf("invalid state transition: %s -> %s", oldState, newState)).
			WithMetadata("device_id", deviceID).
			WithMetadata("old_state", string(oldState)).
			WithMetadata("new_state", string(newState))
	}

	// Create transition record
	transition := &DeviceTransition{
		DeviceID:  deviceID,
		OldState:  oldState,
		NewState:  newState,
		Reason:    reason,
		Event:     event,
		Timestamp: time.Now(),
	}

	sm.logger.Info("disk state transition",
		"device_id", deviceID,
		"old_state", oldState,
		"new_state", newState,
		"reason", reason)

	return transition, nil
}

// GetNextStates returns all valid next states from the current state
func (sm *StateMachine) GetNextStates(currentState types.DiskState) []types.DiskState {
	next, exists := sm.transitions[currentState]
	if !exists {
		return []types.DiskState{}
	}

	// Return a copy to prevent modification
	result := make([]types.DiskState, len(next))
	copy(result, next)
	return result
}

// DetermineStateFromEvent determines the appropriate state based on a udev event
func (sm *StateMachine) DetermineStateFromEvent(
	event *UdevEvent,
	currentState types.DiskState,
) types.DiskState {
	switch event.Action {
	case UdevActionAdd:
		// New device added
		if currentState == types.DiskStateUnknown || currentState == types.DiskStateOffline {
			return types.DiskStateDiscovered
		}
		// Device already known, no state change
		return currentState

	case UdevActionRemove:
		// Device removed
		return types.DiskStateOffline

	case UdevActionChange:
		// Device changed - may indicate status update
		// Preserve current state, trigger re-evaluation
		return currentState

	default:
		return currentState
	}
}

// ValidateStateTransition validates and logs a state transition
func (sm *StateMachine) ValidateStateTransition(
	deviceID string,
	oldState types.DiskState,
	newState types.DiskState,
) error {
	if !sm.CanTransition(oldState, newState) {
		return errors.New(errors.OperationFailed,
			fmt.Sprintf("invalid state transition: %s -> %s", oldState, newState)).
			WithMetadata("device_id", deviceID).
			WithMetadata("old_state", string(oldState)).
			WithMetadata("new_state", string(newState))
	}

	return nil
}
