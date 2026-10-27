package main

import (
	"strings"
	"testing"
	"time"
)

func TestTimerStartPauseResumeReset(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	timer := newFocusTimer()
	if timer.view() != "Focus  25:00" || timer.running {
		t.Fatal("timer must start stopped at 25:00")
	}
	if timer.toggle(now) == nil || !timer.running || timer.deadline != now.Add(focusDuration) {
		t.Fatal("start must set deadline and schedule a tick")
	}
	oldTick := timer.tickID
	pausedAt := now.Add(75*time.Second + 250*time.Millisecond)
	if timer.toggle(pausedAt) != nil || timer.running {
		t.Fatal("pause must stop without scheduling a tick")
	}
	want := focusDuration - 75*time.Second - 250*time.Millisecond
	if timer.remaining != want || timer.view() != "Focus  23:45 · Paused" {
		t.Fatalf("pause lost elapsed time: %s", timer.view())
	}
	if timer.tick(oldTick, now.Add(time.Hour)) != nil || timer.remaining != want {
		t.Fatal("a tick queued before pausing must be ignored")
	}
	resumeAt := now.Add(2 * time.Hour)
	if timer.toggle(resumeAt) == nil || timer.deadline != resumeAt.Add(want) {
		t.Fatal("resume must retain remaining time, excluding the pause")
	}
	if timer.tick(oldTick, resumeAt) != nil {
		t.Fatal("old tick must not restart a second loop after resuming")
	}
	if timer.tick(timer.tickID, resumeAt.Add(30*time.Second)) == nil || timer.remaining != want-30*time.Second {
		t.Fatal("resumed timer should measure elapsed time")
	}
	oldTick = timer.tickID
	timer.reset()
	if timer.running || timer.remaining != focusDuration || timer.view() != "Focus  25:00" {
		t.Fatal("reset must restore 25:00 and stop")
	}
	timer.toggle(resumeAt)
	if timer.tick(oldTick, resumeAt.Add(time.Minute)) != nil || timer.remaining != focusDuration {
		t.Fatal("a tick queued before reset must be ignored after restarting")
	}
}

func TestTimerDelayedAndDuplicateTicks(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	timer := newFocusTimer()
	timer.toggle(now)
	id := timer.tickID
	if timer.tick(id, now.Add(93*time.Second)) == nil || timer.remaining != focusDuration-93*time.Second {
		t.Fatal("delayed updates must account for all elapsed time")
	}
	nextID := timer.tickID
	if timer.tick(id, now.Add(94*time.Second)) != nil || timer.tickID != nextID {
		t.Fatal("duplicate tick must not schedule another successor")
	}
}

func TestTimerCompletion(t *testing.T) {
	for _, elapsed := range []time.Duration{focusDuration, focusDuration + time.Hour} {
		t.Run(elapsed.String(), func(t *testing.T) {
			now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
			timer := newFocusTimer()
			timer.toggle(now)
			id := timer.tickID
			if timer.tick(id, now.Add(elapsed)) != nil || timer.running || timer.remaining != 0 {
				t.Fatal("completed timer must stop at zero")
			}
			if timer.view() != "Focus  00:00 · Session complete" {
				t.Fatalf("unexpected completion display: %s", timer.view())
			}
			if timer.tick(id, now.Add(elapsed)) != nil || timer.toggle(now.Add(elapsed)) != nil {
				t.Fatal("completed session must not schedule ticks until reset")
			}
			timer.reset()
			if timer.toggle(now.Add(elapsed)) == nil || !timer.running {
				t.Fatal("reset must allow a new session")
			}
		})
	}
}

func TestPauseAfterDeadlineCompletes(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	timer := newFocusTimer()
	timer.toggle(now)
	timer.toggle(now.Add(focusDuration + time.Second))
	if timer.running || timer.remaining != 0 || !strings.Contains(timer.view(), "Session complete") {
		t.Fatal("pausing after a delayed final tick must complete the session")
	}
}
