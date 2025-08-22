package lz

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// ParserConfig generates  new parser instances.^.
type ParserConfig interface {
	NewParser() (p Parser, err error)
	BufConfig() BufConfig
	SetBufConfig(bcfg BufConfig)
	json.Marshaler
	json.Unmarshaler
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
	if err := unmarshalJSON(pcfg, data); err != nil {
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

func unmarshalJSON(pcfg ParserConfig, data []byte) error {
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

func marshalJSON(pcfg ParserConfig) (p []byte, err error) {
	m := make(map[string]any)
	m["Type"] = parserType(pcfg)
	v := reflect.Indirect(reflect.ValueOf(pcfg))
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		m[f.Name] = v.Field(i).Interface()
	}
	return json.MarshalIndent(m, "", "  ")
}

func iVal(v reflect.Value, name string) int {
	return int(v.FieldByName(name).Int())
}

func setIVal(v reflect.Value, name string, i int) {
	v.FieldByName(name).SetInt(int64(i))
}

// bufConfig reads the BufConfig from the parser configuration.
func bufConfig(pcfg ParserConfig) BufConfig {
	v := reflect.Indirect(reflect.ValueOf(pcfg))
	bc := BufConfig{
		ShrinkSize: iVal(v, "ShrinkSize"),
		BufferSize: iVal(v, "BufferSize"),
		WindowSize: iVal(v, "WindowSize"),
		BlockSize:  iVal(v, "BlockSize"),
	}
	return bc
}

func setBufConfig(pcfg ParserConfig, bc BufConfig) {
	v := reflect.Indirect(reflect.ValueOf(pcfg))
	setIVal(v, "ShrinkSize", bc.ShrinkSize)
	setIVal(v, "BufferSize", bc.BufferSize)
	setIVal(v, "WindowSize", bc.WindowSize)
	setIVal(v, "BlockSize", bc.BlockSize)
}
