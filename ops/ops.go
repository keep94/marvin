// Package ops contains building blocks for basic operation of hue lights
package ops

import (
	"errors"
	"github.com/keep94/gohue"
	"github.com/keep94/gohue/actions"
	"github.com/keep94/marvin/lights"
	"github.com/keep94/maybe"
	"github.com/keep94/tasks"
	"time"
)

const (
	// The start of hue task Ids from persistent storage. Hard-coded hue tasks
	// must have ids less than this.
	PersistentTaskIdOffset = 10000
)

// Interface Context represents a connection to the hue bridge.
type Context interface {

	// Sets the properties for a particular light
	Set(lightId int, properties *gohue.LightProperties) (
		response []byte, err error)
}

// HueAction represents an action to be done with hue lights.
type HueAction interface {
	// Do does the action.
	// ctxt is the connection to the hue bridge; lightSet is the exact set of
	// lights. The tasks package provides e.
	// If a Do implementation needs more than the Context interface and
	// ctxt does not implement it then Do does nothing.
	Do(ctxt Context, lightSet lights.Set, e *tasks.Execution)

	// UsedLights returns the lights this instance will use given an initial
	// set of lights.
	// Implementations of UsedLights must obey these axioms:
	// 1. UsedLights(UsedLights(A)) == UsedLights(A)
	// 2. If A subset of B then UsedLights(A) subset of UsedLights(B)
	UsedLights(lightSet lights.Set) lights.Set
}

// HueTask represents a HueAction with an ID and description.
// These instances must be treated as immutable.
type HueTask struct {
	Id int
	HueAction
	Description string
}

// Refresh returns this instance.
func (h *HueTask) Refresh() *HueTask {
	return h
}

// GetDescription returns the description of this instance.
func (h *HueTask) GetDescription() string {
	return h.Description
}

// AtTimeTask represents a hue task scheduled to run at a particular time
// on a particular set of lights.
// These instances must be treated as immutable.
type AtTimeTask struct {
	// The schedule Id
	Id string

	// The Hue Task
	H *HueTask

	// The lights to run on
	Ls lights.Set

	// The time to start
	StartTime time.Time
}

// HueTaskList represents an immutable list of hue tasks.
type HueTaskList []*HueTask

// ColorBrightness represents a color and brightness.
type ColorBrightness struct {
	Color      gohue.MaybeColor
	Brightness maybe.Uint8
}

// LightColors represents both color and brightness for each light. The key
// of the map is the light id; the value is the color and brightness for that
// light. A color and brightness for light id 0 means all lights are to have
// that color and brightness.
// These instances must be treated as immutable.
type LightColors map[int]ColorBrightness

// Interface LightReader reads the state of a light
type LightReader interface {
	Get(lightId int) (*gohue.LightProperties, []byte, error)
}

// Snapshot reads the current state of the lights in lightSet.
func Snapshot(reader LightReader, lightSet lights.Set) (LightColors, error) {
	result := make(LightColors, len(lightSet))
	for lightId, valid := range lightSet {
		if !valid {
			continue
		}
		properties, response, err := reader.Get(lightId)
		if err != nil {
			return nil, FixError(lightId, response, err)
		}
		var colorBrightness ColorBrightness
		if properties.On.Value {
			colorBrightness.Color = properties.C
			colorBrightness.Brightness = properties.Bri
		}
		result[lightId] = colorBrightness
	}
	return result, nil
}

// Restore restores the lights back to their original state.
// ctxt is the current context; lightColors are the state of the lights
// as returned by Snapshot.
func Restore(ctxt Context, lightColors LightColors) error {
	for id := range lightColors {
		// use 400ms fade in
		if response, err := ctxt.Set(
			id,
			colorBrightnessToLightPropertiesWithTransition(
				lightColors[id], maybe.NewUint16(4))); err != nil {
			return FixError(id, response, err)
		}
	}
	// Wait 500ms for fade in to take effect
	time.Sleep(500 * time.Millisecond)
	return nil
}

// StaticHueAction represents a HueAction that turns each light on to some
// some color and brightness.
// These instances must be treated as immutable.
type StaticHueAction LightColors

func (a StaticHueAction) Do(
	ctxt Context, lightSet lights.Set, e *tasks.Execution) {
	var globalLightProperties *gohue.LightProperties
	if globalCb, ok := a[0]; ok {
		globalLightProperties = colorBrightnessToLightProperties(globalCb)
	}

	ids, ok := lightSet.Slice()
	if !ok {
		return
	}
	if len(ids) == 0 {
		if globalLightProperties == nil {
			panic("Received All lights, but no global color-brightness")
		}
		if response, err := ctxt.Set(0, globalLightProperties); err != nil {
			e.SetError(FixError(0, response, err))
		}
		return
	}

	for _, id := range ids {
		if globalLightProperties != nil {
			if response, err := ctxt.Set(id, globalLightProperties); err != nil {
				e.SetError(FixError(id, response, err))
			}
		} else {
			if response, err := ctxt.Set(id, colorBrightnessToLightProperties(a[id])); err != nil {
				e.SetError(FixError(id, response, err))
			}
		}
	}
}

func (a StaticHueAction) UsedLights(lightSet lights.Set) lights.Set {
	if _, isAll := a[0]; isAll {
		return lightSet
	}
	usedLights := make(lights.Set, len(a))
	for id := range a {
		usedLights[id] = true
	}
	return usedLights.Intersect(lightSet)
}

// NamedColors represents colors for lights by name read from persistent
// storage.
type NamedColors struct {
	Id          int64
	Colors      LightColors
	Description string
}

// AsHueTask converts this instance to a HueTask
func (nc *NamedColors) AsHueTask() *HueTask {
	return &HueTask{
		Id:          int(nc.Id) + PersistentTaskIdOffset,
		HueAction:   StaticHueAction(nc.Colors),
		Description: nc.Description,
	}
}

// Blink takes a sequence of brightnesses and returns what those brighnesses
// should be when they blink. brights are the original brighnesses. magnitude
// is a value between -255 and 255 inclusive that indicates the magnitude of
// the blink. Positive means lights should blink brighter if possible while
// negative means lights should blink dimmer if possible.
func Blink(brights []uint8, magnitude int) []uint8 {
	if magnitude > 255 || magnitude < -255 {
		panic("Magnitude must be between -255 and 255.")
	}
	isUpPreferred := (magnitude >= 0)
	if !isUpPreferred {
		magnitude = -magnitude
	}
	magnitude128 := magnitude
	if magnitude128 > 128 {
		magnitude128 = 128
	}
	downThreshold := magnitude128
	upThreshold := 256 - magnitude128
	isDownFunc := func(x int) bool {
		return x >= downThreshold
	}
	isUpFunc := func(x int) bool {
		return x < upThreshold
	}
	goDownFunc := func(x int) int {
		result := x - magnitude
		if result < 0 {
			return 0
		}
		return result
	}
	goUpFunc := func(x int) int {
		result := x + magnitude
		if result > 255 {
			return 255
		}
		return result
	}
	upCount := 0
	downCount := 0
	for i := range brights {
		if isUpFunc(int(brights[i])) {
			upCount++
		}
		if isDownFunc(int(brights[i])) {
			downCount++
		}
	}
	var testFunc func(int) bool
	var positiveFunc func(int) int
	var negativeFunc func(int) int
	if upCount > downCount {
		testFunc = isUpFunc
		positiveFunc, negativeFunc = goUpFunc, goDownFunc
	} else if upCount < downCount {
		testFunc = isDownFunc
		positiveFunc, negativeFunc = goDownFunc, goUpFunc
	} else if isUpPreferred {
		testFunc = isUpFunc
		positiveFunc, negativeFunc = goUpFunc, goDownFunc
	} else {
		testFunc = isDownFunc
		positiveFunc, negativeFunc = goDownFunc, goUpFunc
	}
	result := make([]uint8, len(brights))
	for i := range brights {
		x := int(brights[i])
		if testFunc(x) {
			x = positiveFunc(x)
		} else {
			x = negativeFunc(x)
		}
		result[i] = uint8(x)
	}
	return result
}

// FixError converts a response from gohue.Get() or gohue.Set() into
// a descriptive error. lightId is the lightId, rawResponse is the
// response from gohue.Get() or gohue.Set(), err is the original
// error from gohue.Get() or gohue.Set()
func FixError(lightId int, rawResponse []byte, err error) error {
	if err == gohue.NoSuchResourceError {
		return &actions.NoSuchLightIdError{LightId: lightId, RawResponse: rawResponse}
	}
	if len(rawResponse) > 0 {
		return errors.New(string(rawResponse))
	}
	return err
}

func colorBrightnessToLightProperties(
	cb ColorBrightness) *gohue.LightProperties {
	var transitionTime maybe.Uint16
	return colorBrightnessToLightPropertiesWithTransition(
		cb, transitionTime)
}

func colorBrightnessToLightPropertiesWithTransition(
	cb ColorBrightness,
	transitionTime maybe.Uint16) *gohue.LightProperties {
	if !cb.Color.Valid && !cb.Brightness.Valid {
		return &gohue.LightProperties{
			On:             maybe.NewBool(false),
			TransitionTime: transitionTime}
	}
	return &gohue.LightProperties{
		C:              cb.Color,
		Bri:            cb.Brightness,
		On:             maybe.NewBool(true),
		TransitionTime: transitionTime}
}
