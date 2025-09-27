package lz

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// ParserConfig provides the interface to parser configurations.
type ParserConfig interface {
	NewParser() (p Parser, err error)
	BufConfig() BufferConfig
	SetBufConfig(bcfg BufferConfig)
	json.Marshaler
	json.Unmarshaler
	Clone() ParserConfig
	SetDefaults()
	Verify() error
}

func ParseJSON(data []byte) (ParserConfig, error) {
	var s = struct{ Type string }{}
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("lz: json data unmarshal error: %w", err)
	}
	var pcfg ParserConfig
	switch s.Type {
	case "HP":
		pcfg = &HPConfig{}
	case "BHP":
		pcfg = &BHPConfig{}
	case "DHP":
		pcfg = &DHPConfig{}
	case "BDHP":
		pcfg = &BDHPConfig{}
	case "BUP":
		pcfg = &BUPConfig{}
	case "GSAP":
		pcfg = &GSAPConfig{}
	case "OSAP":
		pcfg = &OSAPConfig{}
	default:
		return nil, fmt.Errorf("lz: unknown parser type %s", s.Type)
	}
	if err := UnmarshalJSON(pcfg, data); err != nil {
		return nil, err
	}
	return pcfg, nil
}

func parserType(pcfg ParserConfig) string {
	v := reflect.Indirect(reflect.ValueOf(pcfg))
	s := v.Type().Name()
	pt, ok := strings.CutSuffix(s, "Config")
	if !ok {
		panic("parserConfig type name must end with Config")
	}
	return pt
}

// UnmarshalJSON is a helper function that unmarshals the JSON data into the
// parser configuration value provided.
func UnmarshalJSON(pcfg ParserConfig, data []byte) error {
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	x, ok := m["Type"]
	if !ok {
		return fmt.Errorf("lz: json data needs Type member")
	}
	pt, ok := x.(string)
	if !ok {
		return fmt.Errorf("lz: json data Type member must be string")
	}
	ptCfg := parserType(pcfg)
	if ptCfg != pt {
		return fmt.Errorf("lz: json data Type member must be %s, got %s",
			ptCfg, pt)
	}
	v := reflect.Indirect(reflect.ValueOf(pcfg))
	for k, val := range m {
		if k == "Type" {
			continue
		}
		_, ok := v.Type().FieldByName(k)
		if !ok {
			return fmt.Errorf(
				"lz: %sConfig doesn't have field %s", ptCfg, k)
		}
		fv := v.FieldByName(k)
		vj := reflect.ValueOf(val)
		if !vj.Type().ConvertibleTo(fv.Type()) {
			return fmt.Errorf(
				"lz: json data member %s must have type %s, got %s",
				k, fv.Type(), vj.Type())
		}
		fv.Set(vj.Convert(fv.Type()))
	}
	return nil
}

// MarshalJSON is a helper function that marshals the parser configuration
// value provided into JSON data.
func MarshalJSON(pcfg ParserConfig) (p []byte, err error) {
	buf := new(bytes.Buffer)

	v := reflect.Indirect(reflect.ValueOf(pcfg))
	t := v.Type()
	fmt.Fprintf(buf, "{\n  \"Type\": %q,\n", parserType(pcfg))
	n := t.NumField()
	for i := range n {
		f := t.Field(i)
		v, err := json.Marshal(v.Field(i).Interface())
		if err != nil {
			return nil, fmt.Errorf("lz: json marshal error: %w", err)
		}
		fmt.Fprintf(buf, "  %q: %s", f.Name, v)
		if i < n-1 {
			fmt.Fprint(buf, ",\n")
		} else {
			fmt.Fprint(buf, "\n")
		}
	}
	fmt.Fprintf(buf, "}\n")
	return buf.Bytes(), nil
}

func iVal(v reflect.Value, name string) int {
	return int(v.FieldByName(name).Int())
}

func setIVal(v reflect.Value, name string, i int) {
	v.FieldByName(name).SetInt(int64(i))
}

// BufConfig reads the BufferConfig from the parser configuration.
func BufConfig(pcfg ParserConfig) BufferConfig {
	v := reflect.Indirect(reflect.ValueOf(pcfg))
	bc := BufferConfig{
		ShrinkSize: iVal(v, "ShrinkSize"),
		BufferSize: iVal(v, "BufferSize"),
		WindowSize: iVal(v, "WindowSize"),
		BlockSize:  iVal(v, "BlockSize"),
	}
	return bc
}

func SetBufConfig(pcfg ParserConfig, bc BufferConfig) {
	v := reflect.Indirect(reflect.ValueOf(pcfg))
	setIVal(v, "ShrinkSize", bc.ShrinkSize)
	setIVal(v, "BufferSize", bc.BufferSize)
	setIVal(v, "WindowSize", bc.WindowSize)
	setIVal(v, "BlockSize", bc.BlockSize)
}
