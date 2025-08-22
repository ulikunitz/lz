package lz

import "testing"

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
