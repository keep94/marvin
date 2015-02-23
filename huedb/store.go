// Package huedb contains the persistence layer for the hue web app
package huedb

import (
  "errors"
  "github.com/keep94/appcommon/db"
  "github.com/keep94/gofunctional3/consume"
  "github.com/keep94/gofunctional3/functional"
  "github.com/keep94/marvin/lights"
  "github.com/keep94/marvin/ops"
  "github.com/keep94/tasks"
)

var (
  // Indicates that the id does not exist in the database.
  ErrNoSuchId = errors.New("huedb: No such Id.")
  // Indicates that LightColors map has bad values.
  ErrBadLightColors = errors.New("huedb: Bad values in LightColors.")
)

var (
  kNamedColorsToHueTask = functional.NewMapper(
      func(srcPtr, destPtr interface{}) error {
        s, d := srcPtr.(*ops.NamedColors), destPtr.(**ops.HueTask)
        *d = s.AsHueTask()
        return nil
      },
  )
)

type NamedColorsByIdRunner interface {
  // NamedColorsById gets named colors by id.
  NamedColorsById(t db.Transaction, id int64, colors *ops.NamedColors) error
}

type NamedColorsRunner interface {
  // NamedColors gets all named colors.
  NamedColors(t db.Transaction, consumer functional.Consumer) error
}

type AddNamedColorsRunner interface {
  // AddNamedColros adds named colors.
  AddNamedColors(t db.Transaction, colors *ops.NamedColors) error
}

type UpdateNamedColorsRunner interface {
  // UpdateNamedColors updates named colors by id.
  UpdateNamedColors(t db.Transaction, colors *ops.NamedColors) error
}

type RemoveNamedColorsRunner interface {
  // RemoveNamedColors removes named colors by id.
  RemoveNamedColors(t db.Transaction, id int64) error
}

// HueTasks returns all the named colors as hue tasks.
func HueTasks(store NamedColorsRunner) (ops.HueTaskList, error) {
  var tasks ops.HueTaskList
  consumer := functional.MapConsumer(
      consume.AppendTo(&tasks),
      kNamedColorsToHueTask,
      new(ops.NamedColors))
  if err := store.NamedColors(nil, consumer); err != nil {
    return nil, err
  }
  return tasks, nil
}

// HueTaskById returns a hue task for named colors by its Id. If not found
// or if store is nil, returns a Hue task with an action that reports
// ErrNoSuchId.
func HueTaskById(store NamedColorsByIdRunner, hueTaskId int) *ops.HueTask {
  if store == nil {
    return &ops.HueTask{
        Id: hueTaskId, HueAction: errAction{ErrNoSuchId}, Description: "Error"}
  }
  var namedColors ops.NamedColors
  err := store.NamedColorsById(
      nil, int64(hueTaskId - ops.PersistentTaskIdOffset), &namedColors)
  if err != nil {
    return &ops.HueTask{
        Id: hueTaskId, HueAction: errAction{err}, Description: "Error"}
  }
  return namedColors.AsHueTask()
}

// DescriptionMap updates the description of an ops.NamedColors
// read from the database if the id of the ops.NamedColors plus
// utils.PersistentTaskIdOffset is a key in this map. In this case, the
// corresponding value is the new description.
// These instances must be treated as immutable once created.
type DescriptionMap map[int]string

// Filter updates the description of an ops.NamedColors in place.
// ptr is of type *ops.NamedColors.
func (f DescriptionMap) Filter(ptr interface{}) error {
  p := ptr.(*ops.NamedColors)
  desc, ok := f[int(p.Id) + ops.PersistentTaskIdOffset]
  if ok {
    p.Description = desc
  }
  return nil
}

// FixDescriptionByIdRunner returns a new NamedColorsByIdRunner that works
// just like delegate except that if id + utils.PersistentTaskIdOffset is
// in descriptionMap, then the Description field in fetched NamedColors
// instance is replaced by the corresponding value in description map.
func FixDescriptionByIdRunner(
    delegate NamedColorsByIdRunner,
    descriptionMap DescriptionMap) NamedColorsByIdRunner {
  return &fixDescriptionByIdRunner{
      delegate: delegate,
      filter: descriptionMap}
}

// FixDescriptionsRunner returns a new NamedColorsRunner that works
// just like delegate except that if id + utils.PersistentTaskIdOffset is
// in descriptionMap, then the Description field in fetched NamedColors
// instance is replaced by the corresponding value in description map.
func FixDescriptionsRunner(
    delegate NamedColorsRunner,
    descriptionMap DescriptionMap) NamedColorsRunner {
  return &fixDescriptionRunner{
      delegate: delegate,
      filter: descriptionMap}
}

// FutureHueTask creates a HueTask from persistent storage by Id.
type FutureHueTask struct {
  // Id is the HueTaskId
  Id int
  // Description is the description
  Description string
  // Store retrieves from persistent storage.
  Store NamedColorsByIdRunner
}

// Refresh returns the HueTask freshly read from persistent storage.
func (f *FutureHueTask) Refresh() *ops.HueTask {
  result := *HueTaskById(f.Store, f.Id)
  result.Description = f.Description
  return &result
}

// GetDescription returns the description of this instance.
func (f *FutureHueTask) GetDescription() string {
  return f.Description
}

type errAction struct {
  err error
}

func (a errAction) Do(
    ctxt ops.Context, unusedLightSet lights.Set, e *tasks.Execution) {
  e.SetError(a.err)
}

func (a errAction) UsedLights(
    lightSet lights.Set) lights.Set {
  return lightSet
}

type fixDescriptionRunner struct {
  delegate NamedColorsRunner
  filter functional.Filterer
}

func (r *fixDescriptionRunner) NamedColors(
    t db.Transaction, consumer functional.Consumer) error {
  return r.delegate.NamedColors(
      t, functional.FilterConsumer(consumer, r.filter))
}

type fixDescriptionByIdRunner struct {
  delegate NamedColorsByIdRunner
  filter functional.Filterer
}

func (r *fixDescriptionByIdRunner) NamedColorsById(
    t db.Transaction, id int64, namedColors *ops.NamedColors) error {
  if err := r.delegate.NamedColorsById(t, id, namedColors); err != nil {
    return err
  }
  r.filter.Filter(namedColors)
  return nil
}


