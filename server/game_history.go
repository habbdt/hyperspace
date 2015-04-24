package main

import (
	"container/list"
	"sync"
)

type StateMutationFunction func(*GameState) error

type GameHistory struct {
	events *list.List
	mu     sync.Mutex
}

type GameHistoryEntry struct {
	time  uint64
	state *GameState
	event GameEvent
}

const (
	msToKeepHistoryEvents = 10000
)

func CreateGameHistory() *GameHistory {
	events := list.New()

	// create event for initial game state
	now := MakeTimestamp()
	state := CreateGameState(now)
	events.PushBack(&GameHistoryEntry{now, state, nil})

	return &GameHistory{
		events: events,
	}
}

func (h *GameHistory) Run(event GameEvent) *GameState {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.run(event.Time(), event)
}

func (h *GameHistory) Tick() *GameState {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.run(MakeTimestamp(), nil)
}

func (h *GameHistory) CurrentState() *GameState {
	h.mu.Lock()
	defer h.mu.Unlock()

	// return current state
	return h.currentState()
}

// TODO: This method is no longer called. Need to re-enable cleanup.
func (h *GameHistory) GetCurrentStateAndClean() *GameState {
	h.mu.Lock()
	defer h.mu.Unlock()

	// save state
	state := h.currentState()

	// run dead object cleanup
	now := MakeTimestamp()
	h.run(now, &CleanupEvent{now})

	return state
}

func (h *GameHistory) run(t uint64, event GameEvent) *GameState {
	// find the closest prior entry to inject after
	previousEl := h.closestPriorHistoryEntry(t)

	// inject the new entry
	el := h.events.InsertAfter(&GameHistoryEntry{t, nil, event}, previousEl)

	// re-write history from the element forward
	for el != nil {
		curr := el.Value.(*GameHistoryEntry)
		prev := el.Prev().Value.(*GameHistoryEntry)

		// play physics from previous state to time of current el (immutable update, returns new state)
		curr.state = prev.state.Tick(curr.time)

		// apply current event, if defined
		if curr.event != nil {
			// execute the event code (mutable update on state passed in)
			curr.event.Execute(curr.state)
		}

		// proceed to next element
		el = el.Next()
	}

	h.trim()
	return h.currentState()
}

func (h *GameHistory) currentEntry() *GameHistoryEntry {
	return h.events.Back().Value.(*GameHistoryEntry)
}

func (h *GameHistory) currentState() *GameState {
	return h.currentEntry().state
}

func (h *GameHistory) trim() {
	now := MakeTimestamp()
	for el := h.events.Front(); el != nil && (now-el.Value.(*GameHistoryEntry).time) > msToKeepHistoryEvents; el = h.events.Front() {
		h.events.Remove(el)
	}
}

func (h *GameHistory) closestPriorHistoryEntry(t uint64) *list.Element {
	for e := h.events.Back(); e != nil; e = e.Prev() {
		if e.Value.(*GameHistoryEntry).time <= t {
			return e
		}
	}
	return nil
}
