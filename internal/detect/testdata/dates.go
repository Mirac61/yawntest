package tasks

import (
	"time"
	clock "time"
)

func Today() string { return clock.Now().Format("2006-01-02") }

func CurrentYear() int { return time.Now().Year() }

// not flagged below

type Task struct {
	Due time.Time
}

func NewTask() Task { return Task{Due: time.Now().Add(24 * time.Hour)} }

func CreatedAt() time.Time { return time.Now().UTC() }

func Stamp() int64 { return time.Now().Unix() }

func Expired(task Task) bool { return time.Now().After(task.Due) }

func InBerlin(berlin *time.Location) int { return time.Now().In(berlin).Year() }

func Elapsed(work func()) time.Duration {
	start := time.Now()
	work()
	return time.Since(start)
}

func Save(save func(time.Time)) { save(time.Now()) }
