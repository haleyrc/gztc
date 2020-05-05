package gztc

import "time"

func Time(t time.Time) *time.Time {
	return &t
}

func String(s string) *string {
	return &s
}

func Int64(i int64) *int64 {
	return &i
}
