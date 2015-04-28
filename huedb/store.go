// Package huedb contains the persistence layer for the hue web app
package huedb

import (
  "errors"
  "fmt"
  "github.com/keep94/appcommon/db"
  "github.com/keep94/gofunctional3/consume"
  "github.com/keep94/gofunctional3/functional"
  "github.com/keep94/marvin/dynamic"
  "github.com/keep94/marvin/lights"
  "github.com/keep94/marvin/ops"
  "github.com/keep94/tasks"
  "log"
  "time"
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

// EncodedAtTimeTask is the form of ops.AtTimeTask that can be persisted to
// a database.
type EncodedAtTimeTask struct {
  // The unique database dependent numeric ID of this scheduled task.
  Id int64

  // The string ID of this scheduled task. Database independent.
  ScheduleId string

  // The ID of the scheduled hue task.
  HueTaskId int

  // The encoded form of the hue action in the scheduled hue task.
  Action string

  // The description of the scheduled hue task.
  Description string

  // The encoded set of lights on which the scheduled hue task will run.
  LightSet string

  // The time the hue task is to run in seconds after Jan 1 1970 GMT
  Time int64
}

// EncodedAtTimeTaskStore persists EncodedAtTimeTask instances.
type EncodedAtTimeTaskStore interface {

  // AddEncodedAtTimeTask adds a task.
  AddEncodedAtTimeTask(t db.Transaction, task *EncodedAtTimeTask) error

  // RemoveEncodedAtTimeTaskByScheduleId removes a task by schedule id.
  RemoveEncodedAtTimeTaskByScheduleId(
      t db.Transaction, scheduleId string) error

  // EncodedAtTimeTasks fetches all tasks.
  EncodedAtTimeTasks(t db.Transaction, consumer functional.Consumer) error
}

// ActionEncoder converts a hue action to a string.
// hueTaskId is the id of the enclosing hue task;
// action is what is to be encoded.
type ActionEncoder interface {
  Encode(hueTaskId int, action ops.HueAction) (string, error)
}

// ActionDecoder converts a string back to a hue action.
// hueTaskId is the id of the enclosing hue task; encoded is the string form
// of the hue action.
type ActionDecoder interface {
  Decode(hueTaskId int, encoded string) (ops.HueAction, error)
}

// SpecificActionEncoder converts a specific type of hue action to a string.
type SpecificActionEncoder interface {
  Encode(action ops.HueAction) string
}

// SpecificActionDecoder converts a string back to a specific type of
// hue action.
type SpecificActionDecoder interface {
  Decode(encoded string) (ops.HueAction, error)
}

// DynamicHueTaskStore fetches a dynamic.HueTask by Id. If no task can be
// fetched, returns nil.
type DynamicHueTaskStore interface {
  ById(id int) *dynamic.HueTask
}

// NewActionEncoder returns an ActionEncoder.
// The Encode method of the returned ActionEncoder works the following way.
// If hueTaskId < ops.PersistentTaskIdOffset, then Encode uses store to
// look up the HueTask by hueTaskId. Encode delegates to the Factory field
// of the fetched hue task after converting it to a SpecificActionEncoder.
// Encode reports an error if the Factory field cannot be converted to
// a SpecificActionEncoder.
// If hueTaskId >= ops.PersistentTaskIdOffset, then Encode returns the
// empty string with no error.
func NewActionEncoder(store DynamicHueTaskStore) ActionEncoder {
  return basicActionEncoder{store}
}

// NewActionDecoder returns an ActionDecoder. 
// The Decode method of the returned ActionDecoder works the following way.
// If hueTaskId < ops.PersistentTaskIdOffset, then Decode uses store to
// look up the HueTask by hueTaskId. Decode delegates to the Factory field
// of the fetched hue task after converting it to a SpecificActionDecoder.
// Decode reports an error if the Factory field cannot be converted to
// a SpecificActionDecoder.
// If hueTaskId >= ops.PersistentTaskIdOffset, then Decode uses dbStore
// to look up the hue action with id: hueTaskId - ops.PersistentTaskIdOffset.
func NewActionDecoder(
    store DynamicHueTaskStore,
    dbStore NamedColorsByIdRunner) ActionDecoder {
  return &basicActionDecoder{store: store, dbStore: dbStore}
}

type basicActionEncoder struct {
  store DynamicHueTaskStore
}

func (b basicActionEncoder) Encode(
    id int, action ops.HueAction) (string, error) {
  if id >= ops.PersistentTaskIdOffset {
    return "", nil
  }
  task := b.store.ById(id)
  if task == nil {
    return "", errors.New(fmt.Sprintf("No such Dynamic HueTask ID: %d", id))
  }
  encoder, ok := task.Factory.(SpecificActionEncoder)
  if !ok {
    return "", errors.New(fmt.Sprintf(
        "Dynamic HueTask ID doesn't implement SpecificActionEncoder: %d", id))
  }
  return encoder.Encode(action), nil
}

type basicActionDecoder struct {
  store DynamicHueTaskStore
  dbStore NamedColorsByIdRunner
}

func (b *basicActionDecoder) Decode(
    id int, encoded string) (ops.HueAction, error) {
  if id >= ops.PersistentTaskIdOffset {
    var namedColors ops.NamedColors
    if err := b.dbStore.NamedColorsById(
        nil, int64(id - ops.PersistentTaskIdOffset), &namedColors); err != nil {
      return nil, err
    }
    return ops.StaticHueAction(namedColors.Colors), nil
  }
  task := b.store.ById(id)
  if task == nil {
    return nil, errors.New(fmt.Sprintf("No such Dynamic HueTask ID: %d", id))
  }
  decoder, ok := task.Factory.(SpecificActionDecoder)
  if !ok {
    return nil, errors.New(fmt.Sprintf(
            "Dynamic HueTask ID doesn't implement SpecificActionDecoder: %d", id))
  }
  return decoder.Decode(encoded)
}

// AtTimeTaskStore is a store for ops.AtTimeTask instances.
type AtTimeTaskStore struct {
  encoder ActionEncoder
  decoder ActionDecoder
  store EncodedAtTimeTaskStore
  logger *log.Logger
}

// NewAtTimeTaskStore creates and returns a new AtTimeTaskStore ready for use
func NewAtTimeTaskStore(
    encoder ActionEncoder,
    decoder ActionDecoder,
    store EncodedAtTimeTaskStore,
    logger *log.Logger) *AtTimeTaskStore {
  return &AtTimeTaskStore{
      encoder: encoder, decoder: decoder, store: store, logger: logger}
}

// All returns all tasks.
func (s *AtTimeTaskStore) All() []*ops.AtTimeTask {
  var allEncoded []*EncodedAtTimeTask
  consumer := consume.AppendPtrsTo(&allEncoded, nil)
  if err := s.store.EncodedAtTimeTasks(nil, consumer); err != nil {
    s.logger.Println(err)
    return nil
  }
  var result []*ops.AtTimeTask
  var placeholder EncodedAtTimeTask
  consumer = consume.AppendPtrsTo(&result, nil)
  consumer = functional.MapConsumer(
      consumer, functional.NewMapper(s.mapper), &placeholder)
  encodedStream := functional.NewStreamFromPtrs(allEncoded, nil)
  if err := consumer.Consume(encodedStream); err != nil {
    s.logger.Println(err)
    return nil
  }
  return result
}

// Add adds a new scheduled task
func (s *AtTimeTaskStore) Add(task *ops.AtTimeTask) {
  var encoded EncodedAtTimeTask
  var err error
  encoded.Action, err = s.encoder.Encode(task.H.Id, task.H.HueAction)
  if err != nil {
    s.logger.Printf("While encoding hue task %d: %v", task.H.Id, err)
    return
  }
  encoded.ScheduleId = task.Id
  encoded.HueTaskId = task.H.Id
  encoded.Description = task.H.Description
  encoded.LightSet = task.Ls.String()
  encoded.Time = task.StartTime.Unix()
  err = s.store.AddEncodedAtTimeTask(nil, &encoded)
  if err != nil {
    s.logger.Println(err)
  }
}

// Remove removes a scheduled task by id
func (s *AtTimeTaskStore) Remove(scheduleId string) {
  err := s.store.RemoveEncodedAtTimeTaskByScheduleId(nil, scheduleId)
  if err != nil {
    s.logger.Println(err)
  }
}

func (s *AtTimeTaskStore) mapper(srcPtr, destPtr interface{}) error {
  encoded := srcPtr.(*EncodedAtTimeTask)
  dest := destPtr.(*ops.AtTimeTask)
  var err error
  dest.H = &ops.HueTask{
      Id: encoded.HueTaskId,
      Description: encoded.Description,
  }
  dest.H.HueAction, err = s.decoder.Decode(encoded.HueTaskId, encoded.Action)
  if err != nil {
    s.logger.Printf("While decoding hue task %d: %v", encoded.HueTaskId, err)
    return functional.Skipped
  }
  dest.Ls, err = lights.InvString(encoded.LightSet)
  if err != nil {
    s.logger.Printf("Error parsing light set %s", encoded.LightSet)
    return functional.Skipped
  }
  dest.Id = encoded.ScheduleId
  dest.StartTime = time.Unix(encoded.Time, 0)
  return nil
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


