// Package huedb contains the persistence layer for the hue web app
package huedb

import (
  "errors"
  "fmt"
  "github.com/keep94/appcommon/db"
  "github.com/keep94/goconsume"
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

type NamedColorsByIdRunner interface {
  // NamedColorsById gets named colors by id.
  NamedColorsById(t db.Transaction, id int64, colors *ops.NamedColors) error
}

type NamedColorsRunner interface {
  // NamedColors gets all named colors.
  NamedColors(t db.Transaction, consumer goconsume.Consumer) error
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
  consumer := goconsume.AppendTo(&tasks)
  consumer = &namedColorsToHueTaskConsumer{Consumer: consumer}
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

// DescriptionMap maps hue task ids to descriptions. These instances must
// be treated as immutable.
type DescriptionMap map[int]string

type descriptionMapFilter DescriptionMap

func (f descriptionMapFilter) Filter(ptr interface{}) bool {
  p := ptr.(*ops.NamedColors)
  desc, ok := f[int(p.Id) + ops.PersistentTaskIdOffset]
  if ok {
    p.Description = desc
  }
  return true
}

// FixDescriptionByIdRunner returns a new NamedColorsByIdRunner that works
// just like delegate except that for a fetched NamedColors, x,
// if x.Id + utils.PersistentTaskIdOffset is in descriptionMap, then
// x.Description is replaced with the corresponding value in descriptionMap.
func FixDescriptionByIdRunner(
    delegate NamedColorsByIdRunner,
    descriptionMap DescriptionMap) NamedColorsByIdRunner {
  return &fixDescriptionByIdRunner{
      delegate: delegate,
      filter: descriptionMapFilter(descriptionMap)}
}

// FixDescriptionsRunner returns a new NamedColorsRunner that works
// just like delegate except that ifor a fetched NamedColors, x,
// if x.Id + utils.PersistentTaskIdOffset is in descriptionMap, then
// x.Description is replaced with the corresponding value in descriptionMap.
func FixDescriptionsRunner(
    delegate NamedColorsRunner,
    descriptionMap DescriptionMap) NamedColorsRunner {
  return &fixDescriptionRunner{
      delegate: delegate,
      filter: descriptionMapFilter(descriptionMap)}
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

  // The group id.
  GroupId string

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

  // RemoveEncodedAtTimeTaskByScheduleId removes a task by
  // group Id and schedule id.
  RemoveEncodedAtTimeTaskByScheduleId(
      t db.Transaction, groupId, scheduleId string) error

  // EncodedAtTimeTasks fetches all tasks in a particular group.
  EncodedAtTimeTasks(
      t db.Transaction, groupId string, consumer goconsume.Consumer) error
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

// DynamicHueTaskStore fetches a dynamic.HueTask by Id. If no task can be
// fetched, returns nil.
type DynamicHueTaskStore interface {
  ById(id int) *dynamic.HueTask
}

// NewActionEncoder returns an ActionEncoder.
// The Encode method of the returned ActionEncoder works the following way.
// If hueTaskId < ops.PersistentTaskIdOffset, then Encode uses store to
// look up the HueTask by hueTaskId. Encode delegates to the Factory field
// of the fetched hue task after converting it to a dynamic.Encoder.
// Encode reports an error if the Factory field cannot be converted to
// a dynamic.Encoder.
// If hueTaskId >= ops.PersistentTaskIdOffset, then Encode returns the
// empty string with no error.
func NewActionEncoder(store DynamicHueTaskStore) ActionEncoder {
  return basicActionEncoder{store}
}

// NewActionDecoder returns an ActionDecoder. 
// The Decode method of the returned ActionDecoder works the following way.
// If hueTaskId < ops.PersistentTaskIdOffset, then Decode uses store to
// look up the HueTask by hueTaskId. Decode delegates to the Factory field
// of the fetched hue task after converting it to a dynamic.Decoder.
// Decode reports an error if the Factory field cannot be converted to
// a dynamic.Decoder.
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
  encoder, ok := task.Factory.(dynamic.Encoder)
  if !ok {
    return "", errors.New(fmt.Sprintf(
        "Dynamic HueTask ID doesn't implement dynamic.Encoder: %d", id))
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
  decoder, ok := task.Factory.(dynamic.Decoder)
  if !ok {
    return nil, errors.New(fmt.Sprintf(
            "Dynamic HueTask ID doesn't implement dynamic.Decoder: %d", id))
  }
  return decoder.Decode(encoded)
}

// AtTimeTaskStore is a store for ops.AtTimeTask instances.
type AtTimeTaskStore struct {
  encoder ActionEncoder
  decoder ActionDecoder
  store EncodedAtTimeTaskStore
  groupId string
  logger *log.Logger
}

// NewAtTimeTaskStore creates and returns a new AtTimeTaskStore ready for use
func NewAtTimeTaskStore(
    encoder ActionEncoder,
    decoder ActionDecoder,
    store EncodedAtTimeTaskStore,
    groupId string,
    logger *log.Logger) *AtTimeTaskStore {
  return &AtTimeTaskStore{
      encoder: encoder,
      decoder: decoder,
      store: store,
      groupId: groupId,
      logger: logger}
}

// All returns all tasks.
func (s *AtTimeTaskStore) All() []*ops.AtTimeTask {
  var allEncoded []*EncodedAtTimeTask
  consumer := goconsume.AppendPtrsTo(&allEncoded)
  if err := s.store.EncodedAtTimeTasks(nil, s.groupId, consumer); err != nil {
    s.logger.Println(err)
    return nil
  }
  result := make([]*ops.AtTimeTask, len(allEncoded))
  idx := 0
  for i := range allEncoded {
    atask := s.asAtTimeTask(allEncoded[i])
    if atask == nil {
      if err := s.store.RemoveEncodedAtTimeTaskByScheduleId(
          nil, s.groupId, allEncoded[i].ScheduleId); err != nil {
        s.logger.Println(err)
      }
    } else {
      result[idx] = atask
      idx++
    }
  }
  return result[:idx]
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
  encoded.GroupId = s.groupId
  err = s.store.AddEncodedAtTimeTask(nil, &encoded)
  if err != nil {
    s.logger.Println(err)
  }
}

// Remove removes a scheduled task by id
func (s *AtTimeTaskStore) Remove(scheduleId string) {
  err := s.store.RemoveEncodedAtTimeTaskByScheduleId(nil, s.groupId, scheduleId)
  if err != nil {
    s.logger.Println(err)
  }
}

func (s *AtTimeTaskStore) asAtTimeTask(encoded *EncodedAtTimeTask) *ops.AtTimeTask {
  var err error
  resultH := &ops.HueTask{
      Id: encoded.HueTaskId,
      Description: encoded.Description,
  }
  resultH.HueAction, err = s.decoder.Decode(
      encoded.HueTaskId, encoded.Action)
  if err != nil {
    s.logger.Printf("While decoding hue task %d: %v", encoded.HueTaskId, err)
    return nil
  }
  resultLs, err := lights.InvString(encoded.LightSet)
  if err != nil {
    s.logger.Printf("Error parsing light set %s", encoded.LightSet)
    return nil
  }
  return &ops.AtTimeTask{
      Id: encoded.ScheduleId,
      H: resultH,
      Ls: resultLs,
      StartTime: time.Unix(encoded.Time, 0)}
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
  filter descriptionMapFilter
}

func (r *fixDescriptionRunner) NamedColors(
    t db.Transaction, consumer goconsume.Consumer) error {
  consumer = goconsume.ModFilter(
      consumer, r.filter.Filter, (*ops.NamedColors)(nil))
  return r.delegate.NamedColors(t, consumer)
}

type fixDescriptionByIdRunner struct {
  delegate NamedColorsByIdRunner
  filter descriptionMapFilter
}

func (r *fixDescriptionByIdRunner) NamedColorsById(
    t db.Transaction, id int64, namedColors *ops.NamedColors) error {
  if err := r.delegate.NamedColorsById(t, id, namedColors); err != nil {
    return err
  }
  r.filter.Filter(namedColors)
  return nil
}

type namedColorsToHueTaskConsumer struct {
  goconsume.Consumer
  hueTask *ops.HueTask
}

func (n *namedColorsToHueTaskConsumer) Consume(ptr interface{}) {
  goconsume.MustCanConsume(n)
  p := ptr.(*ops.NamedColors)
  n.hueTask = p.AsHueTask()
  n.Consumer.Consume(&n.hueTask)
}
