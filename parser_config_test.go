package lz

import (
	"reflect"
	"testing"
)

func TestParserType(t *testing.T) {
	pcfg := &HPConfig{}
	const want = "HPConfig"
	if got := parserType(pcfg); got != "HP" {
		t.Fatalf("wanted parser type %s, got %s", want, got)
	}
}

func TestParserJSON_2(t *testing.T) {
	const s = `{"Type": "HP","InputLen":3,"HashBits":16}`
	pcfg, err := ParseJSON([]byte(s))
	if err != nil {
		t.Fatalf("ParseJSON2() error = %v", err)
	}
	if got := parserType(pcfg); got != "HP" {
		t.Fatalf("wanted parser type %s, got %s", "HP", got)
	}
	hpCfg, ok := pcfg.(*HPConfig)
	if !ok {
		t.Fatalf("ParseJSON2() returned wrong type: %T", pcfg)
	}
	if hpCfg.InputLen != 3 {
		t.Fatalf("InputLen is %d, want %d", hpCfg.InputLen, 3)
	}
	if hpCfg.HashBits != 16 {
		t.Fatalf("HashBits are %d, want %d", hpCfg.HashBits, 16)
	}
}

func TestMarshalJSON(t *testing.T) {
	tc := []ParserConfig{
		&HPConfig{
			InputLen: 3,
			HashBits: 16,
		},
		&BDHPConfig{
			InputLen1: 4,
		},
		&BHPConfig{
			BufferSize: 64,
		},
		&BUPConfig{
			WindowSize: 128,
		},
		&DHPConfig{
			BlockSize: 256,
		},
		&GSAPConfig{
			WindowSize: 8 << 20,
		},
		&OSAPConfig{
			ShrinkSize: 512,
		},
	}

	for _, cfg := range tc {
		typ := parserType(cfg)
		t.Run(typ+"Config", func(t *testing.T) {
			type defaultsSetter interface {
				setDefaults()
			}
			cfg.(defaultsSetter).setDefaults()
			data, err := cfg.MarshalJSON()
			if err != nil {
				t.Fatalf("MarshalJSON() error = %v", err)
			}
			t.Logf("json: \n%s", data)
			pcfg, err := ParseJSON(data)
			if err != nil {
				t.Fatalf("ParseJSON() error = %v", err)
			}
			if got := parserType(pcfg); got != typ {
				t.Fatalf("wanted parser type %s, got %s", typ, got)
			}
			if !reflect.DeepEqual(cfg, pcfg) {
				t.Fatalf("wanted config %v, got %v", cfg, pcfg)
			}
		})
	}
}
