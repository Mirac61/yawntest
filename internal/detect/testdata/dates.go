package tasks

import (
	"time"
	clock "time"
)

type Task struct {
	Due time.Time
}

func NewTask() Task {
	return Task{Due: time.Now().Add(24 * time.Hour)}
}

func Today() string { return clock.Now().Format("2006-01-02") }

// not flagged below

func CreatedAt() time.Time { return time.Now().UTC() }

func Stamp() int64 { return time.Now().Unix() }

func Expired(task Task) bool { return time.Now().After(task.Due) }

func InBerlin(berlin *time.Location) time.Time { return time.Now().In(berlin) }
