package config_test

import (
	"github.com/dns4acme/dns4acme/internal/config"
	"reflect"
	"testing"
)

type NestedStruct struct {
	FieldE string `config:"fielde"`
}

type NestedMap map[string]*NestedStruct

type configStruct struct {
	FieldA string `config:"fielda"`
	FieldB int    `config:"fieldb"`
	FieldC struct {
		FieldD string `config:"fieldd"`
	} `config:"fieldc"`

	NestedMap
}

func TestParser_Apply(t *testing.T) {
	var cfg *configStruct
	parser := config.New(&cfg)
	if err := parser.Apply([]string{"fielda"}, reflect.ValueOf("Hello world!")); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	if cfg.FieldA != "Hello world!" {
		t.Fatalf("Incorrect field A: %s", cfg.FieldA)
	}

	if err := parser.Apply([]string{"fieldc", "fieldd"}, reflect.ValueOf("Hello world!")); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	if cfg.FieldC.FieldD != "Hello world!" {
		t.Fatalf("Incorrect field D: %s", cfg.FieldA)
	}
}

func TestParser_Apply_NestedMap(t *testing.T) {
	cfg := &configStruct{}
	cfg.NestedMap = NestedMap{
		"test1": &NestedStruct{FieldE: ""},
	}
	parser := config.New(&cfg)
	if err := parser.Apply([]string{"test1", "fielde"}, reflect.ValueOf("Hello world!")); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	if cfg.NestedMap["test1"].FieldE != "Hello world!" {
		t.Fatalf("Incorrect field E: %s", cfg.NestedMap["test1"].FieldE)
	}
}
