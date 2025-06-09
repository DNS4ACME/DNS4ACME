package config

import (
	"net/netip"
	"reflect"
	"testing"
)

type testStruct1 struct {
	value string
}

func (t *testStruct1) UnmarshalText(data []byte) error {
	t.value = string(data)
	return nil
}

func TestUnmarshalTextConverter_string(t *testing.T) {
	source := "Hello world!"
	target := &testStruct1{}
	conv := unmarshalTextConverter{}
	if err := conv.convert(reflect.ValueOf(source), reflect.ValueOf(target)); err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}
	if target.value != source {
		t.Fatalf("Incorrect target value: %s, expected: %s", target.value, source)
	}
}

func TestUnmarshalTextConverter_string_ptr(t *testing.T) {
	source := "Hello world!"
	target := &testStruct1{}
	conv := unmarshalTextConverter{}
	if err := conv.convert(reflect.ValueOf(&source), reflect.ValueOf(target)); err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}
	if target.value != source {
		t.Fatalf("Incorrect target value: %s, expected: %s", target.value, source)
	}
}

func TestUnmarshalTextConverter_HostPort(t *testing.T) {
	source := "0.0.0.0:53"
	var target *netip.AddrPort
	conv := unmarshalTextConverter{}
	if err := conv.convert(reflect.ValueOf(&source), reflect.ValueOf(&target).Elem()); err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}
	if target == nil {
		t.Fatalf("Nil target")
	}
	if target.Addr().String() != "0.0.0.0" {
		t.Fatalf("Incorrect target value: %s, expected: %s", target.Addr().String(), "0.0.0.0")
	}
	if target.Port() != 53 {
		t.Fatalf("Incorrect target port: %d, expected: %d", target.Port(), 53)
	}
}
