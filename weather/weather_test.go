package weather_test

import (
	"github.com/keep94/marvin/weather"
	"testing"
)

func TestGet(t *testing.T) {
	_, err := weather.Get("AAAA")
	if err == nil {
		t.Error("Expected non-nil error.")
	}
	_, err = weather.Get("KNUQ")
	if err != nil {
		t.Fatalf("Expected nil error, got %v", err)
	}
}

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
