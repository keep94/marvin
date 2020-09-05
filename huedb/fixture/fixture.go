// Package fixture provides test suites to test implementations of the
// interfaces in the huedb package.
package fixture

import (
	"github.com/keep94/goconsume"
	"github.com/keep94/gohue"
	"github.com/keep94/marvin/huedb"
	"github.com/keep94/marvin/ops"
	"github.com/keep94/maybe"
	"reflect"
	"testing"
)

var (
	kFirstNamedColor = &ops.NamedColors{
		Description: "Foo",
		Colors: ops.LightColors{
			3: {gohue.NewMaybeColor(gohue.NewColor(0.5, 0.3)), maybe.NewUint8(98)},
			5: {gohue.NewMaybeColor(gohue.NewColor(0.6, 0.4)), maybe.NewUint8(0)},

			6: {gohue.MaybeColor{}, maybe.Uint8{}}},
	}
	kSecondNamedColor = &ops.NamedColors{
		Description: "Bar",
		Colors: ops.LightColors{
			2: {gohue.NewMaybeColor(gohue.NewColor(0.22, 0.39)), maybe.NewUint8(255)},
			7: {gohue.NewMaybeColor(gohue.NewColor(0.58, 0.41)), maybe.NewUint8(35)},
		},
	}
)

type MinimalStore interface {
	huedb.AddNamedColorsRunner
	huedb.NamedColorsByIdRunner
}

type NamedColorsStore interface {
	MinimalStore
	huedb.NamedColorsRunner
}

type UpdateNamedColorsStore interface {
	MinimalStore
	huedb.UpdateNamedColorsRunner
}

type RemoveNamedColorsStore interface {
	MinimalStore
	huedb.RemoveNamedColorsRunner
}

func NamedColorsById(t *testing.T, store MinimalStore) {
	var first, second, firstResult, secondResult ops.NamedColors
	createNamedColors(t, store, &first, &second)
	if err := store.NamedColorsById(nil, first.Id, &firstResult); err != nil {
		t.Errorf("Got error reading database by id: %v", err)
	}
	if err := store.NamedColorsById(nil, second.Id, &secondResult); err != nil {
		t.Errorf("Got error reading database by id: %v", err)
	}
	assertNCEqual(t, &first, &firstResult)
	assertNCEqual(t, &second, &secondResult)
}

func NamedColors(t *testing.T, store NamedColorsStore) {
	var first, second ops.NamedColors
	createNamedColors(t, store, &first, &second)
	var results []ops.NamedColors
	if err := store.NamedColors(nil, goconsume.AppendTo(&results)); err != nil {
		t.Errorf("Got error reading database: %v", err)
	}
	if out := len(results); out != 2 {
		t.Fatalf("Expected array of size 2, got %d", out)
	}
	assertNCEqual(t, &first, &results[0])
	assertNCEqual(t, &second, &results[1])
}

func UpdateNamedColors(t *testing.T, store UpdateNamedColorsStore) {
	var first, second, firstResult, secondResult ops.NamedColors
	createNamedColors(t, store, &first, &second)
	second.Description = "Green"
	second.Colors = ops.LightColors{
		14: {gohue.NewMaybeColor(gohue.NewColor(0.6, 0.57)), maybe.NewUint8(17)}}
	if err := store.UpdateNamedColors(nil, &second); err != nil {
		t.Errorf("Got error updating database: %v", err)
	}
	if err := store.NamedColorsById(nil, first.Id, &firstResult); err != nil {
		t.Errorf("Got error reading database by id: %v", err)
	}
	if err := store.NamedColorsById(nil, second.Id, &secondResult); err != nil {
		t.Errorf("Got error reading database by id: %v", err)
	}
	assertNCEqual(t, &first, &firstResult)
	assertNCEqual(t, &second, &secondResult)

	// No colors
	second.Colors = nil
	if err := store.UpdateNamedColors(nil, &second); err != nil {
		t.Errorf("Got error updating database: %v", err)
	}
	if err := store.NamedColorsById(nil, second.Id, &secondResult); err != nil {
		t.Errorf("Got error reading database by id: %v", err)
	}
	assertNCEqual(t, &second, &secondResult)

	// Invalid colors
	second.Colors = ops.LightColors{
		-1: {gohue.NewMaybeColor(gohue.NewColor(0.29, 0.29)), maybe.NewUint8(99)}}
	if err := store.UpdateNamedColors(nil, &second); err == nil {
		t.Error("Expected to get an error because of invalid light Id")
	}
	second.Colors = ops.LightColors{
		35: {gohue.NewMaybeColor(gohue.NewColor(1.29, 0.27)), maybe.NewUint8(101)}}
	if err := store.UpdateNamedColors(nil, &second); err == nil {
		t.Error("Expected to get an error because of invalid color")
	}
}

func RemoveNamedColors(t *testing.T, store RemoveNamedColorsStore) {
	var first, second, firstResult, secondResult ops.NamedColors
	createNamedColors(t, store, &first, &second)
	if err := store.RemoveNamedColors(nil, first.Id); err != nil {
		t.Errorf("Got error removing from database: %v", err)
	}
	if err := store.NamedColorsById(
		nil, first.Id, &firstResult); err != huedb.ErrNoSuchId {
		t.Errorf("Expected huedb.ErrNoSuchId, got %v", err)
	}
	if err := store.NamedColorsById(
		nil, second.Id, &secondResult); err != nil {
		t.Errorf("Got error reading database by id: %v", err)
	}
	assertNCEqual(t, &second, &secondResult)
}

func createNamedColors(
	t *testing.T,
	store MinimalStore,
	first *ops.NamedColors,
	second *ops.NamedColors) {
	createNamedColor(t, store, kFirstNamedColor, first)
	createNamedColor(t, store, kSecondNamedColor, second)
}

func createNamedColor(
	t *testing.T,
	store MinimalStore,
	toBeAdded *ops.NamedColors,
	result *ops.NamedColors) {
	*result = *toBeAdded
	if err := store.AddNamedColors(nil, result); err != nil {
		t.Fatalf("Got %v adding to store", err)
	}
	if result.Id == 0 {
		t.Error("Expected Id to be set.")
	}
}

func assertNCEqual(t *testing.T, expected, actual *ops.NamedColors) {
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
}
