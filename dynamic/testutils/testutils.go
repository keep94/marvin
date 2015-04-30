package testutils

import (
  "github.com/keep94/marvin/dynamic"
  "github.com/keep94/marvin/ops"
  "reflect"
  "testing"
)

type encoderDecoder interface {
  Encode(action ops.HueAction) string
  Decode(encoded string) (ops.HueAction, error)
}

// VerifySerialization verifies that action can be serialized and
// deserialized via factory.
func VerifySerialization(
    t *testing.T, factory dynamic.Factory, action ops.HueAction) {
  ed := factory.(encoderDecoder)
  encoded := ed.Encode(action)
  decoded, err := ed.Decode(encoded)
  if err != nil || !reflect.DeepEqual(action, decoded) {
    t.Errorf("Decode failed.")
  }
}
