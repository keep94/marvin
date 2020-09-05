package recurring_test

import (
	"github.com/keep94/marvin/recurring"
	tasks_recurring "github.com/keep94/tasks/recurring"
	"testing"
	"time"
)

var (
	kLocation *time.Location
)

func TestEachSunset(t *testing.T) {
	r := recurring.EachSunset(40.0, -120.0)
	stream := r.ForTime(time.Date(2013, 1, 7, 16, 51, 0, 0, kLocation))
	var atime time.Time
	stream.Next(&atime)
	verifyTime(t, time.Date(2013, 1, 7, 16, 51, 59, 0, kLocation), atime)
	stream.Next(&atime)
	verifyTime(t, time.Date(2013, 1, 8, 16, 52, 57, 0, kLocation), atime)

	stream = r.ForTime(time.Date(2013, 1, 7, 16, 52, 0, 0, kLocation))
	stream.Next(&atime)
	verifyTime(t, time.Date(2013, 1, 8, 16, 52, 57, 0, kLocation), atime)
	stream.Next(&atime)
	verifyTime(t, time.Date(2013, 1, 9, 16, 53, 57, 0, kLocation), atime)
}

func TestOnOrBefore(t *testing.T) {
	startTime := time.Date(2013, 10, 24, 21, 13, 0, 0, kLocation)
	r := tasks_recurring.AtInterval(startTime, 6*time.Hour)
	r = recurring.OnOrBefore(r, 21, 13)
	var atime time.Time
	stream := r.ForTime(startTime)
	stream.Next(&atime)
	verifyTime(t, time.Date(2013, 10, 25, 9, 13, 0, 0, kLocation), atime)
	stream.Next(&atime)
	verifyTime(t, time.Date(2013, 10, 25, 15, 13, 0, 0, kLocation), atime)
	stream.Next(&atime)
	verifyTime(t, time.Date(2013, 10, 25, 21, 13, 0, 0, kLocation), atime)
	stream.Next(&atime)
	verifyTime(t, time.Date(2013, 10, 26, 9, 13, 0, 0, kLocation), atime)
}

func TestOnOrBefore2(t *testing.T) {
	startTime := time.Date(2013, 10, 24, 21, 12, 0, 0, kLocation)
	r := tasks_recurring.AtInterval(startTime, 6*time.Hour)
	r = recurring.OnOrBefore(r, 21, 13)
	var atime time.Time
	stream := r.ForTime(startTime)
	stream.Next(&atime)
	verifyTime(t, time.Date(2013, 10, 24, 21, 13, 0, 0, kLocation), atime)
	stream.Next(&atime)
	verifyTime(t, time.Date(2013, 10, 25, 15, 12, 0, 0, kLocation), atime)
	stream.Next(&atime)
	verifyTime(t, time.Date(2013, 10, 25, 21, 12, 0, 0, kLocation), atime)
	stream.Next(&atime)
	verifyTime(t, time.Date(2013, 10, 25, 21, 13, 0, 0, kLocation), atime)
}

func TestOnOrBefore3(t *testing.T) {
	startTime := time.Date(2013, 10, 24, 21, 14, 0, 0, kLocation)
	r := tasks_recurring.AtInterval(startTime, 6*time.Hour)
	r = recurring.OnOrBefore(r, 21, 13)
	var atime time.Time
	stream := r.ForTime(startTime)
	stream.Next(&atime)
	verifyTime(t, time.Date(2013, 10, 25, 9, 14, 0, 0, kLocation), atime)
	stream.Next(&atime)
	verifyTime(t, time.Date(2013, 10, 25, 15, 14, 0, 0, kLocation), atime)
	stream.Next(&atime)
	verifyTime(t, time.Date(2013, 10, 25, 21, 13, 0, 0, kLocation), atime)
	stream.Next(&atime)
	verifyTime(t, time.Date(2013, 10, 26, 9, 14, 0, 0, kLocation), atime)
}

func TestOnOrBefore4(t *testing.T) {
	startTime := time.Date(2013, 10, 24, 9, 13, 0, 0, kLocation)
	r := tasks_recurring.AtInterval(startTime, 6*time.Hour)
	r = recurring.OnOrBefore(r, 9, 13)
	var atime time.Time
	stream := r.ForTime(startTime)
	stream.Next(&atime)
	verifyTime(t, time.Date(2013, 10, 24, 21, 13, 0, 0, kLocation), atime)
	stream.Next(&atime)
	verifyTime(t, time.Date(2013, 10, 25, 3, 13, 0, 0, kLocation), atime)
	stream.Next(&atime)
	verifyTime(t, time.Date(2013, 10, 25, 9, 13, 0, 0, kLocation), atime)
	stream.Next(&atime)
	verifyTime(t, time.Date(2013, 10, 25, 21, 13, 0, 0, kLocation), atime)
}

func TestOnOrBefore5(t *testing.T) {
	startTime := time.Date(2013, 10, 24, 9, 12, 0, 0, kLocation)
	r := tasks_recurring.AtInterval(startTime, 6*time.Hour)
	r = recurring.OnOrBefore(r, 9, 13)
	var atime time.Time
	stream := r.ForTime(startTime)
	stream.Next(&atime)
	verifyTime(t, time.Date(2013, 10, 24, 9, 13, 0, 0, kLocation), atime)
	stream.Next(&atime)
	verifyTime(t, time.Date(2013, 10, 25, 3, 12, 0, 0, kLocation), atime)
	stream.Next(&atime)
	verifyTime(t, time.Date(2013, 10, 25, 9, 12, 0, 0, kLocation), atime)
	stream.Next(&atime)
	verifyTime(t, time.Date(2013, 10, 25, 9, 13, 0, 0, kLocation), atime)
}

func TestOnOrBefore6(t *testing.T) {
	startTime := time.Date(2013, 10, 24, 9, 14, 35, 451, kLocation)
	r := tasks_recurring.AtInterval(startTime, 6*time.Hour)
	r = recurring.OnOrBefore(r, 9, 13)
	var atime time.Time
	stream := r.ForTime(startTime)
	stream.Next(&atime)
	verifyTime(t, time.Date(2013, 10, 24, 21, 14, 35, 451, kLocation), atime)
	stream.Next(&atime)
	verifyTime(t, time.Date(2013, 10, 25, 3, 14, 35, 451, kLocation), atime)
	stream.Next(&atime)
	verifyTime(t, time.Date(2013, 10, 25, 9, 13, 0, 0, kLocation), atime)
	stream.Next(&atime)
	verifyTime(t, time.Date(2013, 10, 25, 21, 14, 35, 451, kLocation), atime)
}

func verifyTime(t *testing.T, expected, actual time.Time) {
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
}

func init() {
	kLocation, _ = time.LoadLocation("America/Los_Angeles")
}
