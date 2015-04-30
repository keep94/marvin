package testutils

import (
  "github.com/keep94/marvin/dynamic"
  "github.com/keep94/marvin/ops"
  "reflect"
  "testing"
)

// VerifySerialization verifies that action can be serialized and
// deserialized via factory.
func VerifySerialization(
    t *testing.T, factory dynamic.Factory, action ops.HueAction) {
  VerifySerializationWithName(t, "", factory, action)
}

// VerifySerializationWithName verifies that action can be serialized and
// deserialized via factory. The name is displayed in the test failue.
func VerifySerializationWithName(
    t *testing.T, name string, factory dynamic.Factory, action ops.HueAction) {
  ed := factory.(dynamic.FactoryEncoderDecoder)
  encoded := ed.Encode(action)
  decoded, err := ed.Decode(encoded)
  if err != nil || !reflect.DeepEqual(action, decoded) {
    t.Errorf("%s: Decode failed.", name)
  }
}
