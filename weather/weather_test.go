package weather_test

import (
	"testing"

	"github.com/keep94/marvin/weather"
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
