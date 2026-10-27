package main

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
)

const focusDuration = 25 * time.Minute

type timerTickMsg struct{ id uint64 }

type focusTimer struct {
	remaining time.Duration
	deadline  time.Time
	running   bool
	tickID    uint64
}

func newFocusTimer() focusTimer {
	return focusTimer{remaining: focusDuration}
}

func (t *focusTimer) toggle(now time.Time) tea.Cmd {
	if t.running {
		t.remaining = max(0, t.deadline.Sub(now))
		t.running = false
		t.tickID++ // Invalidate the pending tick when pausing.
		return nil
	}
	if t.remaining == 0 {
		return nil // A completed session must be reset before starting again.
	}
	t.deadline = now.Add(t.remaining)
	t.running = true
	return t.nextTick()
}

func (t *focusTimer) reset() {
	t.remaining = focusDuration
	t.running = false
	t.deadline = time.Time{}
	t.tickID++
}

func (t *focusTimer) tick(id uint64, now time.Time) tea.Cmd {
	if !t.running || id != t.tickID {
		return nil
	}
	// Measure elapsed time instead of subtracting one second per update.
	t.remaining = max(0, t.deadline.Sub(now))
	if t.remaining == 0 {
		t.running = false
		t.tickID++
		return nil
	}
	return t.nextTick()
}

func (t *focusTimer) nextTick() tea.Cmd {
	t.tickID++ // Each tick may schedule exactly one successor.
	id := t.tickID
	return tea.Tick(min(time.Second, t.remaining), func(time.Time) tea.Msg {
		return timerTickMsg{id: id}
	})
}

func (t focusTimer) view() string {
	// Round up so a newly started session still displays 25:00.
	seconds := (t.remaining + time.Second - 1) / time.Second
	text := fmt.Sprintf("Focus  %02d:%02d", seconds/60, seconds%60)
	if t.remaining == 0 {
		text += " · Session complete"
	} else if !t.running && t.remaining < focusDuration {
		text += " · Paused"
	}
	return text
}
