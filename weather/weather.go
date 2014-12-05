// Package weather provides current weather conditions
package weather

import (
  "code.google.com/p/go-charset/charset"
  "encoding/xml"
  "fmt"
  "net/http"
  "net/url"
  "sync"
)

// Observation represents a weather observation. 
// These instances must be treated as immutable.
type Observation struct {
  // Temperature in celsius
  Temperature float64 `xml:"temp_c"`
  // Weather conditions e.g 'Fair' or 'Partly Cloudy'
  Weather string `xml:"weather"`
}

// Get returns the current observation from a NOAA weather station. For example
// "KNUQ" means moffett field.
func Get(station string) (observation *Observation, err error) {
  request := &http.Request{
      Method: "GET",
      URL: getUrl(station)}
  var client http.Client
  var resp *http.Response
  if resp, err = client.Do(request); err != nil {
    return
  }
  defer resp.Body.Close()
  decoder := xml.NewDecoder(resp.Body)
  decoder.CharsetReader = charset.NewReader
  var result Observation
  if err = decoder.Decode(&result); err != nil {
    return
  }
  return &result, nil
}

// Cache stores a single weather observation and notifies clients when
// this observation changes. Cache instances can be safely used with
// multiple goroutines.
type Cache struct {
  lock sync.Mutex
  observation *Observation
  stale chan struct{}
}

// NewCache creates a new cache containing no observation.
func NewCache() *Cache {
  return &Cache{stale: make(chan struct{})}
}

// Set updates the observation in this cache and notifies all waiting clients.
func (c *Cache) Set(observation *Observation) {
  close(c.set(observation, make(chan struct{})))
}

// Get returns the current observation in this cache. Clients can use the
// returned channel to block until a new observation is available.
func (c *Cache) Get() (*Observation, <-chan struct{}) {
  c.lock.Lock()
  defer c.lock.Unlock()
  return c.observation, c.stale
}

// Close frees resources associated with this cache.
func (c *Cache) Close() error {
  close(c.set(nil, nil))
  return nil
}

func (c *Cache) set(
    observation *Observation, stale chan struct{}) chan struct{} {
  c.lock.Lock()
  defer c.lock.Unlock()
  c.observation = observation
  result := c.stale
  c.stale = stale
  return result
}

func getUrl(station string) *url.URL {
  return &url.URL{
      Scheme: "http",
      Host: "w1.weather.gov",
      Path: fmt.Sprintf("/xml/current_obs/%s.xml", station)}
}

