package dynamic_test

import (
  "github.com/keep94/gohue"
  "github.com/keep94/marvin/dynamic"
  "github.com/keep94/marvin/ops"
  "github.com/keep94/maybe"
  "net/url"
  "reflect"
  "testing"
)

func TestInt(t *testing.T) {
  param := dynamic.Int(-5, 3, 1, 4)
  if param.MaxCharCount() != 4 {
    t.Error("Expected 4 for MaxCharCount")
  }
  if param.Selection() != nil {
    t.Error("Expected nil for Selection")
  }
  val, str := param.Convert("2")
  assertIntParamValue(t, 2, "2", val, str)
  val, str = param.Convert("3")
  assertIntParamValue(t, 3, "3", val, str)
  val, str = param.Convert("-5")
  assertIntParamValue(t, -5, "-5", val, str)
  val, str = param.Convert("-6")
  assertIntParamValue(t, 1, "1", val, str)
  val, str = param.Convert("4")
  assertIntParamValue(t, 1, "1", val, str)
  val, str = param.Convert("")
  assertIntParamValue(t, 1, "1", val, str)
}

func TestPicker(t *testing.T) {
  choiceList := dynamic.ChoiceList{
      {"Red", 30},
      {"Green", 59},
      {"Blue", 11},
  }
  param := dynamic.Picker(choiceList, 21, "XXI")
  if param.MaxCharCount() != 0 {
    t.Error("Expected 0 for MaxCharCount")
  }
  expectedSelection := []string{"--Pick one--", "Red", "Green", "Blue"}
  actualSelection := param.Selection()
  if !reflect.DeepEqual(expectedSelection, actualSelection) {
    t.Errorf("Expected %v, got %v", expectedSelection, actualSelection)
  }
  val, str := param.Convert("1")
  assertIntParamValue(t, 30, "Red", val, str)
  val, str = param.Convert("2")
  assertIntParamValue(t, 59, "Green", val, str)
  val, str = param.Convert("3")
  assertIntParamValue(t, 11, "Blue", val, str)
  val, str = param.Convert("0")
  assertIntParamValue(t, 21, "XXI", val, str)
  val, str = param.Convert("4")
  assertIntParamValue(t, 21, "XXI", val, str)
  val, str = param.Convert("")
  assertIntParamValue(t, 21, "XXI", val, str)
} 

func TestConstant(t *testing.T) {
  anAction := ops.StaticHueAction{
      0: {gohue.NewMaybeColor(gohue.Blue), maybe.NewUint8(87)}}
  aTask := &dynamic.HueTask{
      Id: 112,
      Description: "Baz",
      Factory: dynamic.Constant(anAction),
  }

  urlValues := make(url.Values)
  expected := &ops.HueTask{
      Id: 112,
      Description: "Baz",
      HueAction: ops.StaticHueAction{
          0: {gohue.NewMaybeColor(gohue.Blue), maybe.NewUint8(87)},
      },
  }
  actual := aTask.FromUrlValues("p", urlValues)
  if !reflect.DeepEqual(expected, actual) {
    t.Errorf("Expected %v, got %v", expected, actual)
  }
}

func TestFromUrlValues(t *testing.T) {
  // TODO: find a way to make this test less fragile.
  // right now it depends on ordering of color chooser and ordering of params.
  // We assume Red is the first color in the chooser and the color is
  // the first param and brightness is the second.
  aTask := &dynamic.HueTask{
      Id: 105,
      Description: "Foo",
      Factory: dynamic.PlainFactory{},
  }
  urlValues := make(url.Values)
  // Color red is first in chooser
  urlValues.Set("p0", "1")
  // Brightness
  urlValues.Set("p1", "98")
  expected := &ops.HueTask{
      Id: 105,
      Description: "Foo Color: Red Bri: 98",
      HueAction: ops.StaticHueAction{
          0: {gohue.NewMaybeColor(gohue.Red), maybe.NewUint8(98)},
      },
  }
  actual := aTask.FromUrlValues("p", urlValues)
  if !reflect.DeepEqual(expected, actual) {
    t.Errorf("Expected %v, got %v", expected, actual)
  }

  // Test defaults
  expected = &ops.HueTask{
      Id: 105,
      Description: "Foo Color: White Bri: 255",
      HueAction: ops.StaticHueAction{
          0: {gohue.NewMaybeColor(gohue.White), maybe.NewUint8(gohue.Bright)},
      },
  }
  // No supplied values
  actual = aTask.FromUrlValues("p", make(url.Values))
  if !reflect.DeepEqual(expected, actual) {
    t.Errorf("Expected %v, got %v", expected, actual)
  }
}

func TestPlainFactoryNewExplicit(t *testing.T) {
  aTask := &dynamic.HueTask{
      Id: 107,
      Description: "Bar",
      Factory: dynamic.PlainFactory{},
  }
  expected := &ops.HueTask{
      Id: 107,
      Description: "Bar Color: Blue Bri: 131",
      HueAction: ops.StaticHueAction{
          0: {gohue.NewMaybeColor(gohue.Blue), maybe.NewUint8(131)},
      },
  }
  actual := aTask.FromExplicit(
      aTask.Factory.(dynamic.PlainFactory).NewExplicit(gohue.Blue, "Blue", 131))
  if !reflect.DeepEqual(expected, actual) {
    t.Errorf("Expected %v, got %v", expected, actual)
  }
}

func TestPlainColorFactoryNewExplicit(t *testing.T) {
  aTask := &dynamic.HueTask{
      Id: 108,
      Description: "Baz",
      Factory: dynamic.PlainColorFactory{gohue.Pink},
  }
  expected := &ops.HueTask{
      Id: 108,
      Description: "Baz Bri: 52",
      HueAction: ops.StaticHueAction{
          0: {gohue.NewMaybeColor(gohue.Pink), maybe.NewUint8(52)},
      },
  }
  actual := aTask.FromExplicit(
      aTask.Factory.(dynamic.PlainColorFactory).NewExplicit(52))
  if !reflect.DeepEqual(expected, actual) {
    t.Errorf("Expected %v, got %v", expected, actual)
  }
}

func TestSortByDescriptionIgnoreCase(t *testing.T) {
  origHueTasks := dynamic.HueTaskList{
      {Id: 10, Description: "Go"},
      {Id: 5, Description: "george"},
      {Id: 7, Description: "abby"},
  }
  expected := dynamic.HueTaskList{
      {Id: 7, Description: "abby"},
      {Id: 5, Description: "george"},
      {Id: 10, Description: "Go"},
  }
  actual := origHueTasks.SortByDescriptionIgnoreCase()
  if !reflect.DeepEqual(expected, actual) {
    t.Errorf("Expected %v, got %v", expected, actual)
  }
}

func assertIntParamValue(
    t *testing.T, eval int, estr string, val interface{}, str string) { 
  if val.(int) != eval {
    t.Errorf("Expected %d, got %d", eval, val.(int))
  }
  if estr != str {
    t.Errorf("Expected %s, got %s", estr, str)
  }
}

