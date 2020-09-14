// Package weather provides current weather conditions
package weather

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"sync"

	"github.com/keep94/appcommon/http_util"
	"golang.org/x/net/html/charset"
)

// Report represents a weather report which may include readings from
// multiple services.
type Report struct {
	// Temperature in celsius
	Temperature float64

	// Weather conditions e.g 'Fair' or 'Partly Cloudy'
	Condition string

	// The Air Quality Index (0-500)
	AQI int
}

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
		URL:    getUrl(station)}
	var client http.Client
	var resp *http.Response
	if resp, err = client.Do(request); err != nil {
		return
	}
	defer resp.Body.Close()
	decoder := xml.NewDecoder(resp.Body)
	decoder.CharsetReader = charset.NewReaderLabel
	var result Observation
	if err = decoder.Decode(&result); err != nil {
		return
	}
	return &result, nil
}

// OpenWeatherConn represents a connection to the open weather servers
type OpenWeatherConn struct {
	client http.Client
	url    *url.URL
}

// NewOpenWeatherConn returns a new, long lived, open weather connection.
func NewOpenWeatherConn(apiKey string) *OpenWeatherConn {
	return &OpenWeatherConn{url: getOpenWeatherUrl(apiKey)}
}

// Get returns the weather for a particular city. The city ID for a city
// can be found by downloading city.list.json.gz from
// http://bulk.openweathermap.org/sample/. For example, Mountain View, CA
// is "5375480"
func (c *OpenWeatherConn) Get(cityId string) (
	observation *Observation, err error) {
	request := &http.Request{
		Method: "GET",
		URL:    http_util.AppendParams(c.url, "id", cityId)}
	var resp *http.Response
	if resp, err = c.client.Do(request); err != nil {
		return
	}
	defer resp.Body.Close()
	decoder := json.NewDecoder(resp.Body)
	var result openWeatherObservation
	if err = decoder.Decode(&result); err != nil {
		return
	}
	if len(result.Weather) == 0 {
		err = errors.New("weather:Missing weather section in open weather response")
		return
	}
	if result.Main == nil {
		err = errors.New("weather:Missing main section in open weather response")
		return
	}
	return &Observation{
		Temperature: result.Main.Temp - 273.15,
		Weather:     result.Weather[0].Description,
	}, nil
}

// PurpleAirConn represents a connection to purple air
type PurpleAirConn struct {
	client http.Client
	url    *url.URL
}

var kPurpleAirConn = &PurpleAirConn{url: getPurpleAirUrl()}

// NewPurpleAirConn returns a new, long lived, purple air connection.
func NewPurpleAirConn() *PurpleAirConn {
	return kPurpleAirConn
}

// GetAQI returns the AQI for a particular purple air station.
func (p *PurpleAirConn) GetAQI(stationId int64) (aqi int, err error) {
	request := &http.Request{
		Method: "GET",
		URL: http_util.AppendParams(
			p.url, "show", strconv.FormatInt(stationId, 10))}
	var resp *http.Response
	if resp, err = p.client.Do(request); err != nil {
		return
	}
	defer resp.Body.Close()
	decoder := json.NewDecoder(resp.Body)
	var result purpleAirResponse
	if err = decoder.Decode(&result); err != nil {
		return
	}
	pm2_5, err := result.AveragePM2_5()
	if err != nil {
		return
	}
	return computeAQI(pm2_5), nil
}

// ReportCache stores a single weather report and notifies clients when
// this report changes. ReportCache instances can be safely used with
// multiple goroutines.
type ReportCache struct {
	lock   sync.Mutex
	report Report
	stale  chan struct{}
}

// NewReportCache creates a new report cache containing a zero value report.
func NewReportCache() *ReportCache {
	return &ReportCache{stale: make(chan struct{})}
}

// Set updates the report in this report cache and notifies all waiting clients.
func (r *ReportCache) Set(report *Report) {
	close(r.set(report, make(chan struct{})))
}

// Get returns a shallow copy of the current report. Clients can use the
// returned channel to block until a new report is available.
func (r *ReportCache) Get() (*Report, <-chan struct{}) {
	r.lock.Lock()
	defer r.lock.Unlock()
	result := r.report
	return &result, r.stale
}

// Close frees resources associated with this report cache.
func (r *ReportCache) Close() error {
	close(r.set(&Report{}, nil))
	return nil
}

func (r *ReportCache) set(
	report *Report, stale chan struct{}) chan struct{} {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.report = *report
	result := r.stale
	r.stale = stale
	return result
}

// Cache stores a single weather observation and notifies clients when
// this observation changes. Cache instances can be safely used with
// multiple goroutines.
type Cache struct {
	lock        sync.Mutex
	observation *Observation
	stale       chan struct{}
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
		Host:   "w1.weather.gov",
		Path:   fmt.Sprintf("/xml/current_obs/%s.xml", station)}
}

func getPurpleAirUrl() *url.URL {
	return &url.URL{
		Scheme: "http",
		Host:   "www.purpleair.com",
		Path:   "/json"}
}

func getOpenWeatherUrl(apiKey string) *url.URL {
	base := &url.URL{
		Scheme: "http",
		Host:   "api.openweathermap.org",
		Path:   "/data/2.5/weather"}
	return http_util.AppendParams(base, "appid", apiKey)
}

type openWeatherObservation struct {
	Weather []openWeatherWeather `json:"weather"`
	Main    *openWeatherMain     `json:"main"`
}

type openWeatherWeather struct {
	Description string `json:"description"`
}

type openWeatherMain struct {
	Temp float64 `json:"temp"`
}

type purpleAirResponse struct {
	Results []purpleAirStation `json:"results"`
}

func (p *purpleAirResponse) AveragePM2_5() (float64, error) {
	sum := 0.0
	count := 0
	for i := range p.Results {
		pm2_5, err := strconv.ParseFloat(p.Results[i].PM2_5, 64)
		if err != nil {
			continue
		}
		sum += pm2_5
		count++
	}
	if count == 0 {
		return 0.0, errors.New("No sensor readings found.")
	}
	return sum / float64(count), nil
}

type purpleAirStation struct {
	ID    int
	PM2_5 string `json:"PM2_5Value"`
}

type aqiEntry struct {
	AQI float64
	Raw float64
}

type aqiScale []aqiEntry

var kAqiScale = aqiScale{
	{0.0, 0.0},
	{50.0, 12.0},
	{51.0, 12.1},
	{100.0, 35.4},
	{101.0, 35.5},
	{150.0, 55.4},
	{151.0, 55.5},
	{200.0, 150.4},
	{201.0, 150.5},
	{300.0, 250.4},
	{301.0, 250.5},
	{400.0, 350.4},
	{401.0, 350.5},
	{500.0, 500.4},
}

func (s aqiScale) GetAQI(raw float64) int {
	idx := s.search(raw)
	if idx == len(s) {
		return round(s[idx-1].AQI)
	}
	if idx == 0 {
		return round(s[0].AQI)
	}
	ratio := (raw - s[idx-1].Raw) / (s[idx].Raw - s[idx-1].Raw)
	return round(s[idx-1].AQI + ratio*(s[idx].AQI-s[idx-1].AQI))
}

func (s aqiScale) search(x float64) int {
	return sort.Search(len(s), func(i int) bool {
		return s[i].Raw >= x
	})
}

func computeAQI(raw float64) int {
	return kAqiScale.GetAQI(raw)
}

func round(x float64) int {
	return int(x + 0.5)
}
