// Package dynamic generates ops.HueTsk dynamically based on user input.
package dynamic

import (
  "bytes"
  "encoding/json"
  "errors"
  "fmt"
  "github.com/keep94/gohue"
  "github.com/keep94/marvin/ops"
  "github.com/keep94/maybe"
  "net/url"
  "sort"
  "strconv"
  "strings"
)

const (
  // Default name of color parameter
  ColorParamName = "Color"

  // Default name of brightness parameter
  BrightnessParamName = "Bri"
)

var (
  // Reported if no value exists for a key in ParamSerializer
  ErrNoValue = errors.New("dynamic: No value.")

  errBadValue = errors.New("dynamic: Bad value.")
)

// Interface Param represents a single parameter for generating a ops.HueTask.
type Param interface {

  // Selection returns the options to appear in the choice dialog. The
  // first option is always similar to "Select one." If the value of this
  // parameter is to be inputted in free form, returns nil.
  Selection() []string

  // MaxCharCount returns the maximum character count of this parameter.
  // It is used as a hint to determine how big to make the input text field
  // for the user.
  MaxCharCount() int

  // Convert converts the string the user entered or the ordinal value of
  // the selected option to the actual value of this parameter. The first
  // returned value is the actual value of the parameter; the second
  // returned value is a string representation that is used in the
  // description of the generated ops.HueTask
  Convert(s string) (interface{}, string)
}

// Choice represents a single choice in a choice dialog.
type Choice struct {

  // What the user sees in the choice dialog
  Name string

  // The parameter value attached to this choice
  Value interface{}
}

// ChoiceList is an immutable list of choices.
type ChoiceList []Choice
  
// Picker returns a Param that is presented as a choice dialog.
// choices are the choices user will see exluding the "Select one" choice;
// defaultValue is the value of returned Param if user does not select a
// choice; defaultName is the description of the default value to use in
// generated ops.HueTask descriptions.
func Picker(
    choices ChoiceList, defaultValue interface{}, defaultName string) Param {
  return &picker{
      Choices: choices,
      DefaultValue: defaultValue,
      DefaultName: defaultName,
  }
}

// Int returns an Param that is presented as a text field and has an
// integer value. minValue and maxValue the minimum and maximum value
// inclusive of the integer; defaultValue is the default value if user
// doesn't enter a number or enters one that is out of range; maxChars
// is the size of the text field.
func Int(
    minValue, maxValue, defaultValue, maxChars int) Param {
  return &intParam{
      MinValue: minValue,
      MaxValue: maxValue,
      DefaultValue: defaultValue,
      MaxChars: maxChars,
  }
}

// Brightness is a convenience rourtine that returns an integer parameter
// representing brightness which is (0-255) with default of 255 and size
// of 3 chars.
func Brightness() Param {
  return kBrightness
}

// ColorPicker returns a Param that lets the user choose a color from a
// predefined list. defaultColor is the default color if user does not
// choose; defaultName is the name to show for the default color.
func ColorPicker(defaultColor gohue.Color, defaultName string) Param {
  return Picker(kColorChoices, defaultColor, defaultName)
}
  
// NamedParam represents a Param that is named.
type NamedParam struct {

  // The name which appears on user input forms and in description of
  // generated ops.HueTask.
  Name string
  Param
}

// NamedParamList represents an immutable list of NamedParam
type NamedParamList []NamedParam

// Factory generates an ops.HueAction from a list of user inputs.
// Specific implementations also provide a NewExplicit mehod that takes
// explicitly typed parameters and returns a new ops.HueAction and the
// parameters as strings.
type Factory interface {

  // Params returns the parameters for which user must supply values.
  Params() NamedParamList

  // New creates the ops.HueAction using the values that the user supplied.
  // values will have the same length as what Params returns.
  New(values []interface{}) ops.HueAction
}

// Constant returns a Factory that provides no user inputs and always
// generates the supplied ops.HueAction.
func Constant(a ops.HueAction) Factory {
  return constantFactory{a}
}

// HueTask represents a task that needs user input to generate a real
// ops.HueTask. These instances must be treated as immutable.
type HueTask struct {

  // Unique Id.
  Id int

  // e.g "Fixed color and brightness"
  Description string

  // Helps to generate the ops.HueTask
  Factory
}  

// FromOpsHueTask is a convenience routine that converts an
// ops.HueTask into a HueTask.
func FromOpsHueTask(h *ops.HueTask) *HueTask {
  return &HueTask{
      Id: h.Id,
      Description: h.Description,
      Factory: Constant(h.HueAction),
  }
}

// FromExplicit creates an ops.HueTask from this instance.
// Callers must call NewExplcit on this instance's Factory field and pass
// the return values to this method.
func (h *HueTask) FromExplicit(
    action ops.HueAction, paramsAsStrings []string) *ops.HueTask {
  return &ops.HueTask{
      Id: h.Id,
      Description: h.getDescription(paramsAsStrings),
      HueAction: action,
  }
}

// FromUrlValues generates an ops.HueTask based on url values from an html
// form. prefix is the prefix of url values for example if prefix is "p" then
// user supplied inputs would be under "p0" "p1" "p2" etc; values are the
// url values. FromUrlValues includes the description of this instance along
// with a description of each user supplied parameter in the returned
// ops.HueTask
func (h *HueTask) FromUrlValues(prefix string, values url.Values) *ops.HueTask {
  params := h.Params()
  paramValues := make([]interface{}, len(params))
  paramNames := make([]string, len(params))
  for i := range params {
    paramValues[i], paramNames[i] = params[i].Convert(
        values.Get(fmt.Sprintf("%s%d", prefix, i)))
  }
  return h.FromExplicit(h.New(paramValues), paramNames)
}

func (h *HueTask) getDescription(names []string) string {
  params := h.Params()
  if len(params) == 0 {
    return h.Description
  }
  parts := make([]string, len(params))
  for i := range parts {
    parts[i] = fmt.Sprintf("%s: %s", params[i].Name, names[i])
  } 
  return fmt.Sprintf("%s %s", h.Description, strings.Join(parts, " "))
}

// HueTaskList represents an immutable list of hue tasks.
type HueTaskList []*HueTask

// FromOpsHueTaskList is a convenience routine that converts an
// ops.HueTaskList into a HueTaskList.
func FromOpsHueTaskList(l ops.HueTaskList) HueTaskList {
  result := make(HueTaskList, len(l))
  for i := range l {
    result[i] = FromOpsHueTask(l[i])
  }
  return result
}

// ToMap returns this HueTaskList as a map keyed by Id
func (l HueTaskList) ToMap() map[int]*HueTask {
  result := make(map[int]*HueTask, len(l))
  for _, ht := range l {
    result[ht.Id] = ht
  }
  return result
}

// SortByDescriptionIgnoreCase returns a new HueTaskList with the same
// HueTasks as this instance only sorted by description in ascending order
// ignoring case.
func (l HueTaskList) SortByDescriptionIgnoreCase() HueTaskList {
  result := make(HueTaskList, len(l))
  copy(result, l)
  sort.Sort(byDescriptionIgnoreCase(result))
  return result
}

// ParamSerializer encodes parameters for hue tasks as a string.
type ParamSerializer map[string][]string

// Encode encodes stored parameters as a single string.
func (p ParamSerializer) Encode() string {
  var buffer bytes.Buffer
  encoder := json.NewEncoder(&buffer)
  if  err := encoder.Encode(p); err != nil {
    panic(err)
  }
  return buffer.String()
}

// NewParamSerializer decodes a string back into parameters. Caller can
// safely modify the returned value.
func NewParamSerializer(s string) (ParamSerializer, error) {
  buffer := bytes.NewBufferString(s)
  decoder := json.NewDecoder(buffer)
  var result ParamSerializer
  err := decoder.Decode(&result)
  return result, err
}

// SetInt stores an int value and returns this instance for chaining.
func (p ParamSerializer) SetInt(key string, value int) ParamSerializer {
  p[key] = []string{strconv.Itoa(value)}
  return p
}

// GetInt returns the stored int value. If no int value stored under key then
// returns ErrNoValue.
func (p ParamSerializer) GetInt(key string) (result int, err error) {
  value := p[key]
  if len(value) != 1 {
    err = ErrNoValue
    return
  }
  return strconv.Atoi(value[0])
}

// SetColor stores an color value and returns this instance for chaining.
func (p ParamSerializer) SetColor(key string, color gohue.Color) ParamSerializer {
  x := int(color.X() * 10000.0 + 0.5)
  y := int(color.Y() * 10000.0 + 0.5)
  p[key] = []string{strconv.Itoa(x), strconv.Itoa(y)}
  return p
}

// GetColor returns the stored Color value. If no Color value stored under key
// then returns ErrNoValue.
func (p ParamSerializer) GetColor(key string) (result gohue.Color, err error) {
  value := p[key]
  if len(value) != 2 {
    err = ErrNoValue
    return
  }
  var x, y int
  if x, err = strconv.Atoi(value[0]); err != nil {
    return
  }
  if y, err = strconv.Atoi(value[1]); err != nil {
    return
  }
  if x < 0 || x > 10000 || y < 0 || y > 10000 {
    err = errBadValue
    return
  }
  result = gohue.NewColor(float64(x) / 10000.0, float64(y) / 10000.0)
  return
}

// PlainFactory implements Factory and lets user provide brightness and
// color and then generates an ops.HueAction that makes lights the user
// supplied color and brightness.
// Default is white with full brightness.
type PlainFactory struct {
}

func (p PlainFactory) Params() NamedParamList {
  return kPlainParams
}

func (p PlainFactory) New(values []interface{}) ops.HueAction {
  color := values[0].(gohue.Color)
  brightness := values[1].(int)
  return plainAction(color, uint8(brightness))
}

// color is the light color; colorString is the string representation
// of the light color; brightness is the brightness of the light.
func (p PlainFactory) NewExplicit(
    color gohue.Color,
    colorString string,
    brightness uint8) (action ops.HueAction, paramsAsStrings []string) {
  briStr := strconv.Itoa(int(brightness))
  return plainAction(color, brightness), []string{colorString, briStr}
}

var (
  kPlainParams = NamedParamList {
      {Name: ColorParamName, Param: ColorPicker(gohue.White, "White")},
      {Name: BrightnessParamName, Param: Brightness()},
  }
)

// PlainColorFactory implements Factory and lets user provide brightness
// only then generates an ops.HueAction that makes lights the user
// supplied brightness with given color. Default is full brightness.
type PlainColorFactory struct {
  // The color the light is to have
  Color gohue.Color
}

func (p PlainColorFactory) Params() NamedParamList {
  return kPlainColorParams
}

func (p PlainColorFactory) New(values []interface{}) ops.HueAction {
  brightness := values[0].(int)
  return plainAction(p.Color, uint8(brightness))
}

// brightness is the brightness of the light.
func (p PlainColorFactory) NewExplicit(
    brightness uint8) (action ops.HueAction, paramsAsStrings []string) {
  briStr := strconv.Itoa(int(brightness))
  return plainAction(p.Color, brightness), []string{briStr}
}

func plainAction(color gohue.Color, brightness uint8) ops.HueAction {
  return ops.StaticHueAction{
      0: ops.ColorBrightness{
             gohue.NewMaybeColor(color),
             maybe.NewUint8(brightness),
         },
  }
}

var (
  kPlainColorParams = NamedParamList {
      {Name: BrightnessParamName, Param: Brightness()},
  }
)

var (
  kBrightness = Int(0, 255, 255, 3)
  kColorChoices = ChoiceList{
      {"Red", gohue.Red},
      {"Green", gohue.Green},
      {"Blue", gohue.Blue},
      {"Yellow", gohue.Yellow},
      {"Magenta", gohue.Magenta},
      {"Cyan", gohue.Cyan},
      {"Purple", gohue.Purple},
      {"White", gohue.White},
      {"Pink", gohue.Pink},
      {"Orange", gohue.Orange},
  }
)

type noSelect struct {
}

func (n noSelect) Selection() []string {
  return nil
}

type intParam struct {
  noSelect
  MinValue int
  MaxValue int
  DefaultValue int
  MaxChars int
}

func (p *intParam) MaxCharCount() int {
  return p.MaxChars
}

func (p *intParam) Convert(s string) (interface{}, string) {
  result, err := strconv.Atoi(s)
  if err != nil || result > p.MaxValue || result < p.MinValue {
    result = p.DefaultValue
  }
  return result, strconv.Itoa(result)
}

type picker struct {
  Choices ChoiceList
  DefaultValue interface{}
  DefaultName string
}

func (p *picker) Selection() []string {
  result := make([]string, len(p.Choices) + 1)
  result[0] = "--Pick one--"
  for i := range p.Choices {
    result[i + 1] = p.Choices[i].Name
  }
  return result
}

func (p *picker) MaxCharCount() int {
  return 0
}

func (p *picker) Convert(s string) (interface{}, string) {
  val, _ := strconv.Atoi(s)
  if val < 1 || val > len(p.Choices) {
    return p.DefaultValue, p.DefaultName
  }
  return p.Choices[val - 1].Value, p.Choices[val - 1].Name
}

type constantFactory struct {
  Action ops.HueAction
}

func (f constantFactory) Params() NamedParamList {
  return nil
}

func (f constantFactory) New(values []interface{}) ops.HueAction {
  return f.Action
}

type byDescriptionIgnoreCase HueTaskList

func (a byDescriptionIgnoreCase) Len() int {
  return len(a)
}

func (a byDescriptionIgnoreCase) Swap(i, j int) {
  a[i], a[j] = a[j], a[i]
}

func (a byDescriptionIgnoreCase) Less(i, j int) bool {
  return strings.ToLower(a[i].Description) < strings.ToLower(a[j].Description)
}

