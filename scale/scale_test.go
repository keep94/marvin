package scale_test

import (
	"github.com/keep94/gohue"
	"github.com/keep94/marvin/scale"
	"reflect"
	"testing"
)

var (
	kOne   = scale.Color{{20.0, gohue.Green}}
	kTwo   = scale.Color{{15.0, gohue.Red}, {20.0, gohue.Blue}}
	kThree = scale.Color{
		{15.0, gohue.Green}, {20.0, gohue.Yellow}, {25.0, gohue.Red}}
)

func TestGetWithOne(t *testing.T) {
	assertEqual(t, gohue.Green, kOne.Get(20.0))
	assertEqual(t, gohue.Green, kOne.Get(19.0))
	assertEqual(t, gohue.Green, kOne.Get(21.0))
}

func TestGetWithTwo(t *testing.T) {
	assertEqual(t, gohue.Red, kTwo.Get(14.0))
	assertEqual(t, gohue.Red, kTwo.Get(15.0))
	assertEqual(t, gohue.Blue, kTwo.Get(16.0))
	assertEqual(t, gohue.Blue, kTwo.Get(20.0))
	assertEqual(t, gohue.Blue, kTwo.Get(21.0))
}

func TestGetWithThree(t *testing.T) {
	assertEqual(t, gohue.Green, kThree.Get(14.0))
	assertEqual(t, gohue.Green, kThree.Get(15.0))
	assertEqual(t, gohue.Yellow, kThree.Get(19.0))
	assertEqual(t, gohue.Yellow, kThree.Get(20.0))
	assertEqual(t, gohue.Red, kThree.Get(21.0))
	assertEqual(t, gohue.Red, kThree.Get(25.0))
	assertEqual(t, gohue.Red, kThree.Get(26.0))
}

func TestInterpolateWithOne(t *testing.T) {
	assertEqual(t, gohue.Green, kOne.Interpolate(20.0))
	assertEqual(t, gohue.Green, kOne.Interpolate(19.0))
	assertEqual(t, gohue.Green, kOne.Interpolate(21.0))
}

func TestInterpolateWithTwo(t *testing.T) {
	assertEqual(t, gohue.Red, kTwo.Interpolate(14.0))
	assertEqual(t, gohue.Red, kTwo.Interpolate(15.0))
	assertEqual(t, gohue.Red.Blend(gohue.Blue, 0.2), kTwo.Interpolate(16.0))
	assertEqual(t, gohue.Blue, kTwo.Interpolate(20.0))
	assertEqual(t, gohue.Blue, kTwo.Interpolate(21.0))
}

func TestInterpolateWithThree(t *testing.T) {
	assertEqual(t, gohue.Green, kThree.Interpolate(14.0))
	assertEqual(t, gohue.Green, kThree.Interpolate(15.0))
	assertEqual(
		t, gohue.Green.Blend(gohue.Yellow, 0.8), kThree.Interpolate(19.0))
	assertEqual(t, gohue.Yellow, kThree.Interpolate(20.0))
	assertEqual(
		t, gohue.Yellow.Blend(gohue.Red, 0.2), kThree.Interpolate(21.0))
	assertEqual(t, gohue.Red, kThree.Interpolate(25.0))
	assertEqual(t, gohue.Red, kThree.Interpolate(26.0))
}

func assertEqual(t *testing.T, expected, actual gohue.Color) {
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
}
