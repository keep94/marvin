package ops_test

import (
	"github.com/keep94/gohue"
	"github.com/keep94/marvin/lights"
	"github.com/keep94/marvin/ops"
	"github.com/keep94/maybe"
	"reflect"
	"testing"
)

func TestStaticHueActionUsedLightsAll(t *testing.T) {
	a := ops.StaticHueAction(map[int]ops.ColorBrightness{
		0: {gohue.NewMaybeColor(gohue.Red), maybe.NewUint8(128)}})
	usedLights := a.UsedLights(lights.All)
	if out := usedLights.String(); out != "All" {
		t.Errorf("Expected All got %v", out)
	}
	usedLights = a.UsedLights(lights.New(3, 5))
	if out := usedLights.String(); out != "3,5" {
		t.Errorf("Expected 3,5 got %v", out)
	}
}

func TestStaticHueActionUsedLightsSome(t *testing.T) {
	someColor := gohue.NewMaybeColor(gohue.Red)
	someBrightness := maybe.NewUint8(128)
	a := ops.StaticHueAction(map[int]ops.ColorBrightness{
		1: {someColor, someBrightness},
		2: {someColor, someBrightness},
		3: {someColor, someBrightness}})
	usedLights := a.UsedLights(lights.All)
	if out := usedLights.String(); out != "1,2,3" {
		t.Errorf("Expected 1,2,3 got %v", out)
	}
	usedLights = a.UsedLights(lights.New(2, 3, 4))
	if out := usedLights.String(); out != "2,3" {
		t.Errorf("Expected 2,3 got %v", out)
	}
	usedLights = a.UsedLights(lights.New(4, 5))
	if !usedLights.IsNone() {
		t.Errorf("Expected no lights")
	}
}

func TestStaticHueActionUsedLightsNone(t *testing.T) {
	var a ops.StaticHueAction
	usedLights := a.UsedLights(lights.All)
	if !usedLights.IsNone() {
		t.Error("Expected no lights.")
	}
	usedLights = a.UsedLights(lights.New(2, 3, 4))
	if !usedLights.IsNone() {
		t.Error("Expected no lights.")
	}
}

func TestStaticHueActionDoAll(t *testing.T) {
	someColor := gohue.NewMaybeColor(gohue.Red)
	someBrightness := maybe.NewUint8(128)
	a := ops.StaticHueAction(map[int]ops.ColorBrightness{
		0: {someColor, someBrightness}})
	ctxt := make(contextForTesting)
	a.Do(ctxt, lights.All, nil)
	expected := contextForTesting{
		0: {C: someColor, Bri: someBrightness, On: maybe.NewBool(true)},
	}
	if !reflect.DeepEqual(expected, ctxt) {
		t.Errorf("Expected %v, got %v", expected, ctxt)
	}

	ctxt = make(contextForTesting)
	a.Do(ctxt, lights.New(2, 4), nil)
	expected = contextForTesting{
		2: {C: someColor, Bri: someBrightness, On: maybe.NewBool(true)},
		4: {C: someColor, Bri: someBrightness, On: maybe.NewBool(true)},
	}
	if !reflect.DeepEqual(expected, ctxt) {
		t.Errorf("Expected %v, got %v", expected, ctxt)
	}
}

func TestStaticHueActionDoAllOff(t *testing.T) {
	var noColor gohue.MaybeColor
	var noBrightness maybe.Uint8
	a := ops.StaticHueAction(map[int]ops.ColorBrightness{
		0: {noColor, noBrightness}})
	ctxt := make(contextForTesting)
	a.Do(ctxt, lights.All, nil)
	expected := contextForTesting{
		0: {On: maybe.NewBool(false)},
	}
	if !reflect.DeepEqual(expected, ctxt) {
		t.Errorf("Expected %v, got %v", expected, ctxt)
	}

	ctxt = make(contextForTesting)
	a.Do(ctxt, lights.New(2, 4), nil)
	expected = contextForTesting{
		2: {On: maybe.NewBool(false)},
		4: {On: maybe.NewBool(false)},
	}
	if !reflect.DeepEqual(expected, ctxt) {
		t.Errorf("Expected %v, got %v", expected, ctxt)
	}
}

func TestStaticHueActionDoSome(t *testing.T) {
	var noColor gohue.MaybeColor
	var noBrightness maybe.Uint8
	a := ops.StaticHueAction(map[int]ops.ColorBrightness{
		2: {noColor, noBrightness},
		4: {gohue.NewMaybeColor(gohue.Green), maybe.NewUint8(192)},
		5: {gohue.NewMaybeColor(gohue.Blue), maybe.NewUint8(64)}})
	ctxt := make(contextForTesting)
	a.Do(ctxt, lights.New(2, 5), nil)
	expected := contextForTesting{
		2: {
			On: maybe.NewBool(false),
		},
		5: {
			C:   gohue.NewMaybeColor(gohue.Blue),
			Bri: maybe.NewUint8(64),
			On:  maybe.NewBool(true),
		},
	}
	if !reflect.DeepEqual(expected, ctxt) {
		t.Errorf("Expected %v, got %v", expected, ctxt)
	}
}

func TestBlinkDesiredDirection(t *testing.T) {
	actual := ops.Blink([]uint8{47, 49, 48}, -47)
	expected := []uint8{0, 2, 1}
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
	actual = ops.Blink([]uint8{200, 198, 199}, 55)
	expected = []uint8{255, 253, 254}
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
}

func TestBlinkOppositeDirection(t *testing.T) {
	actual := ops.Blink([]uint8{47, 49, 48}, -48)
	expected := []uint8{95, 97, 96}
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
	actual = ops.Blink([]uint8{200, 198, 199}, 56)
	expected = []uint8{144, 142, 143}
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
}

func TestBlinkBestDirection(t *testing.T) {
	actual := ops.Blink([]uint8{131, 132, 130, 124}, -125)
	expected := []uint8{6, 7, 5, 249}
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
	actual = ops.Blink([]uint8{124, 123, 130, 131}, -125)
	expected = []uint8{249, 248, 255, 6}
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
	actual = ops.Blink([]uint8{124, 123, 130, 131}, 125)
	expected = []uint8{249, 248, 255, 6}
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
}

func TestBlinkOverHalf(t *testing.T) {
	actual := ops.Blink([]uint8{
		0, 255, 127, 128, 126, 125, 130, 129, 131, 124}, -130)
	expected := []uint8{130, 125, 255, 0, 255, 255, 0, 0, 1, 254}
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
	actual = ops.Blink([]uint8{
		0, 255, 127, 128, 126, 125, 130, 129, 131, 124}, 130)
	expected = []uint8{130, 125, 255, 0, 255, 255, 0, 0, 1, 254}
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
}

func TestBlinkFull(t *testing.T) {
	actual := ops.Blink([]uint8{
		0, 1, 127, 128, 129, 255}, 255)
	expected := []uint8{255, 255, 255, 0, 0, 0}
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
	actual = ops.Blink([]uint8{
		0, 1, 127, 128, 129, 255}, -255)
	expected = []uint8{255, 255, 255, 0, 0, 0}
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
}

func TestBlinkZero(t *testing.T) {
	actual := ops.Blink([]uint8{55, 254, 82, 97}, 0)
	expected := []uint8{55, 254, 82, 97}
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
}

type contextForTesting map[int]*gohue.LightProperties

func (c contextForTesting) Set(
	lightId int,
	properties *gohue.LightProperties) (respone []byte, err error) {
	propertiesCopy := *properties
	c[lightId] = &propertiesCopy
	return
}
