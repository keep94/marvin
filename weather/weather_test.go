package weather_test

import (
	"testing"

	"github.com/keep94/marvin/weather"
	asserts "github.com/stretchr/testify/assert"
)

func TestCache(t *testing.T) {
	cache := weather.NewCache()
	defer cache.Close()
	observation, stale := cache.Get()
	if observation != nil {
		t.Error("Expected nil observation")
	}
	go func() {
		cache.Set(&weather.Observation{Temperature: 30.0})
	}()
	<-stale
	observation, stale = cache.Get()
	if observation.Temperature != 30.0 {
		t.Error("Expected 30.0 temperature")
	}
	go func() {
		cache.Set(&weather.Observation{Temperature: 35.0})
	}()
	<-stale
	observation, _ = cache.Get()
	if observation.Temperature != 35.0 {
		t.Error("Expected 35.0 temperature")
	}
}

func TestReportCache(t *testing.T) {
	assert := asserts.New(t)
	cache := weather.NewReportCache()
	defer cache.Close()
	report, stale := cache.Get()
	assert.Zero(*report)
	go func() {
		report := &weather.Report{Temperature: 25.0}
		cache.Set(report)
		report.Temperature = 95.0
	}()
	<-stale
	report, _ = cache.Get()
	assert.Equal(25.0, report.Temperature)
	report.Temperature = 99.0
	report, stale = cache.Get()
	assert.Equal(25.0, report.Temperature)
	go func() {
		cache.Set(&weather.Report{Temperature: 35.0})
	}()
	<-stale
	report, _ = cache.Get()
	assert.Equal(35.0, report.Temperature)
}
