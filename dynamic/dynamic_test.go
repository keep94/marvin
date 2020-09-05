package dynamic_test

import (
	"github.com/keep94/gohue"
	"github.com/keep94/marvin/dynamic"
	"github.com/keep94/marvin/dynamic/testutils"
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
	factory := dynamic.Constant(anAction)
	aTask := &dynamic.HueTask{
		Id:          112,
		Description: "Baz",
		Factory:     factory,
	}
	testutils.VerifySerialization(t, factory, anAction)

	urlValues := make(url.Values)
	expected := &ops.HueTask{
		Id:          112,
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
		Id:          105,
		Description: "Foo",
		Factory:     dynamic.PlainFactory{},
	}
	urlValues := make(url.Values)
	// Color red is first in chooser
	urlValues.Set("p0", "1")
	// Brightness
	urlValues.Set("p1", "98")
	expected := &ops.HueTask{
		Id:          105,
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
		Id:          105,
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
		Id:          107,
		Description: "Bar",
		Factory:     dynamic.PlainFactory{},
	}
	expected := &ops.HueTask{
		Id:          107,
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
	testutils.VerifySerialization(t, aTask.Factory, actual.HueAction)
}

func TestPlainColorFactoryNewExplicit(t *testing.T) {
	aTask := &dynamic.HueTask{
		Id:          108,
		Description: "Baz",
		Factory:     dynamic.PlainColorFactory{gohue.Pink},
	}
	expected := &ops.HueTask{
		Id:          108,
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
	testutils.VerifySerialization(t, aTask.Factory, actual.HueAction)
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

func TestParamSerializerBadValue(t *testing.T) {
	s := `{"bar":["6082","10001"],"baz":["6082", "-1"],"a":["-1","6082"],"b":["6082","10001"],"foo":["a","3"],"c":["3","a"],"d":["l"],"e":["-1"],"f":["256"]}`
	q, err := dynamic.NewParamSerializer(s)
	if err != nil {
		t.Fatal("Got error deserializing.")
	}
	if _, err := q.GetColor("bar"); err == nil || err == dynamic.ErrNoValue {
		t.Error("Expected to get error.")
	}
	if _, err := q.GetColor("baz"); err == nil || err == dynamic.ErrNoValue {
		t.Error("Expected to get error.")
	}
	if _, err := q.GetColor("a"); err == nil || err == dynamic.ErrNoValue {
		t.Error("Expected to get error.")
	}
	if _, err := q.GetColor("b"); err == nil || err == dynamic.ErrNoValue {
		t.Error("Expected to get error.")
	}
	if _, err := q.GetColor("foo"); err == nil || err == dynamic.ErrNoValue {
		t.Error("Expected to get error.")
	}
	if _, err := q.GetColor("c"); err == nil || err == dynamic.ErrNoValue {
		t.Error("Expected to get error.")
	}
	if _, err := q.GetInt("d"); err == nil || err == dynamic.ErrNoValue {
		t.Error("Expected to get error.")
	}
	if _, err := q.GetBrightness("d"); err == nil || err == dynamic.ErrNoValue {
		t.Error("Expected to get error.")
	}
	if _, err := q.GetBrightness("e"); err == nil || err == dynamic.ErrNoValue {
		t.Error("Expected to get error.")
	}
	if _, err := q.GetBrightness("f"); err == nil || err == dynamic.ErrNoValue {
		t.Error("Expected to get error.")
	}
}

func TestParamSerializer(t *testing.T) {
	p := make(dynamic.ParamSerializer)
	p.SetInt("foo", 15).SetInt("bar", 35).SetInt("baz", -55).SetInt("foo", 20)
	p.SetBrightness("dim", 1).SetBrightness("bright", 200)
	p.SetColor("bar", gohue.Red).SetColor("green", gohue.Green)
	p.SetColor("bar", gohue.Orange)
	s := p.SetColor("pink", gohue.Pink).SetInt("pink", 7).Encode()
	q, err := dynamic.NewParamSerializer(s)
	if err != nil {
		t.Fatal("Got error deserializing.")
	}
	if out, err := q.GetInt("foo"); out != 20 || err != nil {
		t.Errorf("Expected 20, got %d", out)
	}
	if out, err := q.GetInt("baz"); out != -55 || err != nil {
		t.Errorf("Expected -55, got %d", out)
	}
	if _, err := q.GetInt("bar"); err == nil || err == dynamic.ErrNoValue {
		t.Errorf("Expected to get an undefined error, got %v", err)
	}
	if _, err := q.GetInt("notthere"); err != dynamic.ErrNoValue {
		t.Errorf("Expected to get ErrNoValue, got %v", err)
	}
	if out, err := q.GetBrightness("dim"); out != 1 || err != nil {
		t.Errorf("Expected 1, got %d", out)
	}
	if out, err := q.GetBrightness("bright"); out != 200 || err != nil {
		t.Errorf("Expected 200, got %d", out)
	}
	if _, err := q.GetBrightness("baz"); err == nil || err == dynamic.ErrNoValue {
		t.Errorf("Expected to get an undefined error, got %v", err)
	}
	if _, err := q.GetBrightness("notthere"); err != dynamic.ErrNoValue {
		t.Errorf("Expected to get ErrNoValue, got %v", err)
	}
	if out, err := q.GetColor("bar"); out != gohue.Orange || err != nil {
		t.Errorf("Expected orange, got %v", out)
	}
	if out, err := q.GetColor("green"); out != gohue.Green || err != nil {
		t.Errorf("Expected green, got %v", out)
	}
	if _, err := q.GetColor("pink"); err == nil || err == dynamic.ErrNoValue {
		t.Errorf("Expected to get an undefined error, got %v", err)
	}
	if _, err := q.GetColor("notthere"); err != dynamic.ErrNoValue {
		t.Errorf("Expected to get ErrNoValue, got %v", err)
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
