package config

import (
	"bytes"
	"fmt"
	"github.com/dns4acme/dns4acme/lang/E"
	"log/slog"
	"reflect"
	"slices"
	"strings"
)

func New(cfg any) *Parser {
	val := reflect.ValueOf(cfg)
	if val.Kind() != reflect.Ptr {
		// TODO better error handling
		panic("cfg must be a pointer")
	}
	result := &Parser{
		Root: val.Elem(),
	}
	if err := result.addTypeOptions(val, nil); err != nil {
		// TODO better error handling
		panic(err)
	}
	return result
}

func parsePath(path string) []string {
	path = strings.ReplaceAll(path, "_", "-")
	path = strings.ReplaceAll(path, ".", "-")
	return strings.Split(path, "-")
}

type Parser struct {
	Root    reflect.Value
	Options []Option
}

func (p *Parser) addTypeOptions(t reflect.Value, path Path) error {
	typedT := reflect.ValueOf(t.Interface())
	switch typedT.Kind() {
	case reflect.Ptr:
		if t.IsNil() {
			newValue := reflect.New(t.Type().Elem()).Interface()
			t.Set(reflect.ValueOf(newValue))
		}
		return p.addTypeOptions(t.Elem(), path)
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			fieldType := t.Type().Field(i)
			configTag := fieldType.Tag.Get("config")
			if configTag != "" {
				newPath := append(path.Copy(), parsePath(configTag)...)
				method := field.MethodByName("UnmarshalText")
				if field.Kind() == reflect.Struct && !method.IsValid() {
					if err := p.addTypeOptions(t.Field(i), newPath); err != nil {
						// TODO better error handling
						return err
					}
				} else {
					if !field.IsValid() || !field.CanSet() {
						// TODO better error handling
						panic("cannot set field " + fieldType.Name)
					}
					p.Options = append(p.Options, Option{
						Path:        newPath,
						Value:       field,
						Field:       fieldType,
						Default:     fieldType.Tag.Get("default"),
						Description: fieldType.Tag.Get("description"),
					})
				}
			} else if fieldType.Anonymous {
				switch (fieldType.Type).Kind() {
				case reflect.Struct:
					if err := p.addTypeOptions(field, path); err != nil {
						// TODO better error handling
						return err
					}
				case reflect.Map:
					if field.Type().Key().Kind() != reflect.String {
						return fmt.Errorf("unsupported type %s", t.Kind().String())
					}
					for _, key := range field.MapKeys() {
						newPath := append(path.Copy(), parsePath(key.String())...)
						// TODO check if the value is addressable, otherwise the Apply will panic.
						if err := p.addTypeOptions(field.MapIndex(key), newPath); err != nil {
							// TODO better error handling
							return err
						}
					}
				default:
					// TODO better error handling
					return fmt.Errorf("unsupported type %s", t.Kind().String())
				}
			}
		}
	default:
		// TODO better error handling
		return fmt.Errorf("unsupported type %s", t.Kind().String())
	}
	return nil
}

func (p *Parser) ApplyDefaults() error {
	for _, opt := range p.Options {
		if opt.Default != "" {
			if err := opt.apply(reflect.ValueOf(opt.Default)); err != nil {
				// TODO better error handling
				return err
			}
		}
	}
	return nil
}

func (p *Parser) ApplyEnv(prefix string, env []string) error {
	for _, variable := range env {
		fields := strings.SplitN(variable, "=", 2)
		if len(fields) != 2 {
			// TODO better error handling
			return fmt.Errorf("invalid environment variable: %s", variable)
		}
		key := fields[0]
		value := fields[1]
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		key = strings.TrimPrefix(key, prefix)
		path := parsePath(key)
		if err := p.Apply(path, reflect.ValueOf(value)); err != nil {
			if !E.Is(err, ErrNoSuchOption) {
				return err
			}
		}
	}
	return nil
}

func (p *Parser) ApplyCMD(cmd []string) error {
	if len(cmd) == 0 {
		// TODO better error handling
		return fmt.Errorf("invalid command line arguments")
	}
	cmd = cmd[1:]

	for len(cmd) > 0 {
		if !strings.HasPrefix(cmd[0], "--") {
			return fmt.Errorf("expected flag, found: %s", cmd[0])
		}
		key := parsePath(strings.TrimPrefix(cmd[0], "--"))
		if len(cmd) > 1 && !strings.HasPrefix(cmd[1], "--") {
			if err := p.Apply(key, reflect.ValueOf(cmd[1])); err != nil {
				// TODO better error handling
				return err
			}
			cmd = cmd[2:]
		} else {
			if err := p.Apply(key, reflect.ValueOf(true)); err != nil {
				// TODO better error handling
				return err
			}
			cmd = cmd[1:]
		}
	}
	return nil
}

func (p *Parser) Apply(path Path, value reflect.Value) error {
	// TODO the performance of this is less than ideal
	for _, opt := range p.Options {
		if !opt.Path.Equals(path) {
			continue
		}
		return opt.apply(value)
	}
	// TODO better error handling
	return ErrNoSuchOption.WithAttr(slog.String("option", path.String()))
}

func (p *Parser) CLIHelp() []byte {
	slices.SortStableFunc(p.Options, func(a, b Option) int {
		return a.Path.Compare(b.Path)
	})
	wr := bytes.NewBuffer(nil)
	options := make([]string, len(p.Options))
	descriptions := make([]string, len(p.Options))
	maxLength := 0
	for i, opt := range p.Options {
		options[i] = "--" + strings.Join(opt.Path, "-")
		descriptions[i] = opt.Description
		maxLength = max(len(options[i]), maxLength)
	}
	padText := ""
	for i := 0; i < maxLength+4; i++ {
		padText += " "
	}
	for i := 0; i < len(options); i++ {
		wr.Write([]byte("  " + options[i] + "  "))
		for j := len(options[i]); j < maxLength; j++ {
			wr.Write([]byte{' '})
		}
		description := wordWrap(descriptions[i], 80-maxLength+4)
		for j, line := range description {
			if j != 0 {
				wr.Write([]byte(padText))
			}
			wr.Write([]byte(line + "\n"))
		}
		if len(description) == 0 {
			wr.Write([]byte("\n"))
		}
	}
	return wr.Bytes()
}

func wordWrap(text string, length int) []string {
	words := strings.Fields(strings.TrimSpace(text))
	if len(words) == 0 {
		return []string{}
	}
	line := ""
	var result []string
	for _, word := range words {
		previousLine := line
		line += " " + word
		if len(line) > length && len(previousLine) != 0 {
			result = append(result, line)
			line = ""
		}
	}
	if len(line) > 0 {
		result = append(result, line)
	}
	return result
}

type Path []string

func (p Path) Compare(other Path) int {
	if len(p) == 0 {
		if len(other) > 0 {
			return -1
		}
		return 0
	} else if len(other) == 0 {
		return 1
	}
	if p[0] != other[0] {
		return strings.Compare(p[0], other[0])
	}
	return p[1:].Compare(other[1:])
}

func (p Path) Copy() Path {
	result := make(Path, len(p))
	copy(result, p)
	return result
}

func (p Path) Equals(other Path) bool {
	return slices.EqualFunc(p, other, strings.EqualFold)
}

func (p Path) String() string {
	return strings.Join(p, "-")
}

type Option struct {
	Path        Path
	Value       reflect.Value
	Field       reflect.StructField
	Default     string
	Description string
}

func (o Option) apply(value reflect.Value) error {
	targetFieldValue := o.Value
	targetType := targetFieldValue.Type()
	if value.Type().AssignableTo(targetType) {
		if !targetFieldValue.CanSet() {
			return fmt.Errorf("cannot assign value to %s", targetType.String())
		}
		targetFieldValue.Set(value)
		return nil
	}
	var lastError error
	for _, conv := range converters {
		if err := conv.convert(value, targetFieldValue); err != nil {
			if E.Is(err, ErrCannotConvertValue) {
				lastError = err
				continue
			}
			return ErrIncompatibleValue.Wrap(err).WithAttr(slog.String("option", o.Path.String())).WithAttr(slog.String("source", value.Type().String())).WithAttr(slog.String("target", targetFieldValue.Type().String()))
		}
		return nil
	}
	return ErrIncompatibleValue.Wrap(lastError).WithAttr(slog.String("option", o.Path.String())).WithAttr(slog.String("source", value.Type().String())).WithAttr(slog.String("target", targetFieldValue.Type().String()))
}
