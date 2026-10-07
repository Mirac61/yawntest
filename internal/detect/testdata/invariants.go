package billing

import "time"

func Split(total int64, parts int) []int64 { return nil }

func SplitFees(fees int, ways int) []int { return nil }

func WeekRange(day time.Time) (time.Time, time.Time) { return day, day }

func Period(day time.Time) (start, end time.Time) { return day, day }

// no invariant below

func SplitFloat(total float64, parts int) []float64 { return nil }

func Chunk(items int, size int) []int { return nil }

func Pair(day time.Time) (time.Time, time.Time) { return day, day }

func Times(day time.Time) (created, updated time.Time) { return day, day }
