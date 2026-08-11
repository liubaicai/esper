package esper

import "time"

// windowPreviousAccessByEvent returns the retained window-access history for
// each event.  Sorted and time-order windows deliberately use their current
// access order here; ordinary windows retain their existing insertion order.
// The per-event map lets downstream expression evaluation keep the same
// snapshot when a single input batch contains both arrivals and removals.
func windowPreviousAccessByEvent(spec WindowSpec, state *windowRuntimeState) map[string][]Event {
	if spec == nil || state == nil {
		return nil
	}
	if window, ok := spec.(GroupWindowSpec); ok {
		result := make(map[string][]Event)
		for _, child := range state.groups {
			for key, history := range windowPreviousAccessByEvent(window.Inner, child) {
				result[key] = append([]Event(nil), history...)
			}
		}
		return result
	}
	switch spec.(type) {
	case TimeOrderWindowSpec, SortedWindowSpec:
		return windowHistoryByEvent(spec, state)
	default:
		return nil
	}
}

// windowPreviousAccessHistoryForEvent returns the current view-access order
// for a new event, including an event that was accepted by the view and then
// immediately evicted. Esper still evaluates PREV-family expressions for
// that insert-stream row against the post-update view. Group windows must
// resolve the child partition before taking the history snapshot.
func windowPreviousAccessHistoryForEvent(spec WindowSpec, state *windowRuntimeState, event Event, now time.Time, variables map[string]Value) []Event {
	if spec == nil || state == nil {
		return nil
	}
	if window, ok := spec.(GroupWindowSpec); ok {
		key := groupWindowKeys(window.effectiveKeys(), event, now, variables)
		return windowPreviousAccessHistoryForEvent(window.Inner, state.groups[key], event, now, variables)
	}
	switch spec.(type) {
	case TimeOrderWindowSpec, SortedWindowSpec:
		return windowHistory(spec, state)
	default:
		return nil
	}
}

func windowUsesPreviousAccess(spec WindowSpec) bool {
	if window, ok := spec.(GroupWindowSpec); ok {
		return windowUsesPreviousAccess(window.Inner)
	}
	switch spec.(type) {
	case TimeOrderWindowSpec, SortedWindowSpec, TimeAccumWindowSpec:
		return true
	default:
		return false
	}
}

func windowUsesArrivalPrior(spec WindowSpec) bool {
	if _, ok := spec.(GroupWindowSpec); ok {
		return true
	}
	switch spec.(type) {
	case TimeOrderWindowSpec, SortedWindowSpec, TimeAccumWindowSpec:
		return true
	default:
		return false
	}
}

func arrivalHistoryThrough(arrival []Event, target Event) []Event {
	for index, event := range arrival {
		if sameEvent(event, target) {
			return append([]Event(nil), arrival[:index+1]...)
		}
	}
	return nil
}

func addPriorHistories(target map[string][]Event, arrival []Event, events []Event) {
	if target == nil || len(arrival) == 0 {
		return
	}
	for _, event := range events {
		if history := arrivalHistoryThrough(arrival, event); history != nil {
			target[eventIdentity(event)] = history
		}
	}
}

func windowPriorHistoryForEvent(spec WindowSpec, state *windowRuntimeState, target Event) []Event {
	if spec == nil || state == nil {
		return nil
	}
	if window, ok := spec.(GroupWindowSpec); ok {
		for _, child := range state.groups {
			if history := windowPriorHistoryForEvent(window.Inner, child, target); history != nil {
				return history
			}
		}
		return nil
	}
	switch spec.(type) {
	case TimeOrderWindowSpec, SortedWindowSpec, TimeAccumWindowSpec:
		return arrivalHistoryThrough(state.arrival, target)
	default:
		return nil
	}
}

func windowPriorAccessByEvent(spec WindowSpec, state *windowRuntimeState) map[string][]Event {
	if spec == nil || state == nil {
		return nil
	}
	if window, ok := spec.(GroupWindowSpec); ok {
		result := make(map[string][]Event)
		for _, child := range state.groups {
			for key, history := range windowPriorAccessByEvent(window.Inner, child) {
				result[key] = append([]Event(nil), history...)
			}
		}
		return result
	}
	switch spec.(type) {
	case TimeOrderWindowSpec, SortedWindowSpec, TimeAccumWindowSpec:
		if len(state.arrival) == 0 {
			return nil
		}
		result := make(map[string][]Event, len(state.arrival))
		for index, event := range state.arrival {
			result[eventIdentity(event)] = append([]Event(nil), state.arrival[:index+1]...)
		}
		return result
	default:
		return nil
	}
}

func removeArrivalEvents(arrival *[]Event, removed []Event) {
	if arrival == nil || len(*arrival) == 0 || len(removed) == 0 {
		return
	}
	for _, target := range removed {
		for index, candidate := range *arrival {
			if !sameEvent(candidate, target) {
				continue
			}
			*arrival = append((*arrival)[:index], (*arrival)[index+1:]...)
			break
		}
	}
}
