package lz

import "fmt"

// Config provides a general method to create sequencers.
type Config struct {
	// MemoryBudget specifies the memory budget in bytes for the sequencer. The
	// budget controls how much memory the sequencer has for the window size and the
	// match search data structures. It doesn't control temporary memory
	// allocations. It is a budget, so it can be overdrawn, right?
	MemoryBudget int
	// Effort is scale from 1 to 10 controlling the CPU consumption. A
	// sequencer with an effort of 1 might be extremely fast but will have a
	// worse compression ratio. The default effort is 6 and will provide a
	// reasonable compromise between compression speed and compression
	// ratio. Effort 10 will provide the best compression ratio but will
	// require a higher compression ratio but will be very slow.
	Effort int
	// MaxBlockSize defines a maximum block size. Note that the configurator
	// might create a smaller block size to fit the match search data
	// structures into the memory budget. The main consumer is ZStandard
	// which has a maximum block size of 128 kByte.
	BlockSize int
	// WindowSize fixes the window size.
	WindowSize int
}

// ApplyDefaults applies the defaults to the Config structure. The memory budget
// is set to 2 MB, the effort to 5 and the block size to 128 kByte unless no
// other non-zero values have been set.
func (cfg *Config) ApplyDefaults() {
	if cfg.MemoryBudget == 0 {
		cfg.MemoryBudget = cfg.WindowSize + 2*1024*1024
	}
	if cfg.Effort == 0 {
		cfg.Effort = 5
	}
	if cfg.BlockSize == 0 {
		cfg.BlockSize = 128 * 1024
	}
	// WindowSize stays 0 if none is given
}

// Verify checks the configuration for errors. Use ApplyDefaults before this
// function because it doesn't support zero values in all cases.
func (cfg *Config) Verify() error {
	if cfg.MemoryBudget <= 0 {
		return fmt.Errorf("lz: cfg.MemoryBudget must be positive")
	}
	if !(1 <= cfg.Effort && cfg.Effort <= 9) {
		return fmt.Errorf("lz: cfg.Effort must be in range 1-9")
	}
	if cfg.BlockSize <= 0 {
		return fmt.Errorf("lz: BlockSize must be positive")
	}
	if cfg.WindowSize < 0 {
		return fmt.Errorf("lz: cfg.WindowSize must be non-negative")
	}
	if cfg.WindowSize > cfg.MemoryBudget {
		return fmt.Errorf("lz: memory budget must be larger" +
			" or equal window size")
	}
	return nil
}

// computeConfig computes the configuration extremely fast.
func computeConfig(cfg *Config) (c Configurator, err error) {
	panic("TODO")
}

func computeConfigWindow(cfg *Config) (c Configurator, err error) {
	panic("TODO")
}

func (cfg Config) computeConfig() (c Configurator, err error) {
	cfg.ApplyDefaults()
	if err = cfg.Verify(); err != nil {
		return nil, err
	}
	if cfg.WindowSize == 0 {
		return computeConfig(&cfg)
	}
	return computeConfigWindow(&cfg)

}

// NewInputSequencer creates a new sequencer according to the parameters
// provided. The function will only return an error the parameters are negative
// but otherwise always try to satisfy the requirements.
func (cfg *Config) NewInputSequencer() (s InputSequencer, err error) {
	c, err := cfg.computeConfig()
	if err != nil {
		return nil, err
	}
	return c.NewInputSequencer()
}

const kb = 1024

type hsParams struct {
	memSize  int
	inputLen int
	bits     int
}

func findHSParams(p []hsParams, m int) hsParams {
	a := 0
	b := len(p) - 1
	for a < b {
		i := (a + b + 1) / 2
		if m < p[i].memSize {
			b = i - 1
			continue
		}
		a = i
	}
	return p[a]
}

type dhsParams struct {
	memSize   int
	inputLen1 int
	bits1     int
	inputLen2 int
	bits2     int
}

func findDHSParams(p []dhsParams, m int) dhsParams {
	a := 0
	b := len(p) - 1
	for a < b {
		i := (a + b + 1) / 2
		if m < p[i].memSize {
			b = i - 1
			continue
		}
		a = i
	}
	return p[a]
}

var hsParameters = []hsParams{
	{64 * kb, 3, 11},
	{128 * kb, 3, 13},
	{256 * kb, 3, 14},
	{384 * kb, 3, 15},
	{640 * kb, 3, 16},
	{2048 * kb, 4, 17},
	{4096 * kb, 4, 18},
	{5120 * kb, 5, 18},
	{6144 * kb, 5, 19},
	{13312 * kb, 5, 20},
	{25600 * kb, 5, 21},
	{50176 * kb, 5, 22},
}

var bhsParameters = []hsParams{
	{64 * kb, 3, 11},
	{128 * kb, 3, 13},
	{256 * kb, 3, 14},
	{384 * kb, 3, 15},
	{960 * kb, 3, 16},
	{2048 * kb, 4, 17},
	{4096 * kb, 4, 18},
	{5120 * kb, 5, 18},
	{7168 * kb, 5, 19},
	{14336 * kb, 5, 20},
	{27548 * kb, 5, 21},
	{54272 * kb, 5, 22},
}

var dhsParameters = []dhsParams{
	{64 * kb, 2, 10, 4, 11},
	{128 * kb, 2, 11, 5, 13},
	{192 * kb, 2, 12, 5, 13},
	{256 * kb, 2, 12, 5, 14},
	{320 * kb, 3, 13, 6, 14},
	{384 * kb, 3, 14, 6, 14},
	{512 * kb, 3, 15, 6, 14},
	{640 * kb, 3, 15, 7, 15},
	{2048 * kb, 3, 16, 7, 16},
	{3072 * kb, 3, 16, 7, 17},
	{4068 * kb, 3, 16, 7, 18},
	{9216 * kb, 3, 16, 8, 19},
	{16384 * kb, 3, 16, 8, 20},
	{27684 * kb, 3, 16, 8, 21},
	{65536 * kb, 3, 16, 8, 22},
}

var bdhsParameters = []dhsParams{
	{64 * kb, 2, 10, 4, 11},
	{128 * kb, 2, 11, 5, 13},
	{256 * kb, 2, 11, 5, 14},
	{320 * kb, 3, 13, 6, 14},
	{512 * kb, 3, 14, 7, 15},
	{640 * kb, 3, 15, 7, 15},
	{2048 * kb, 3, 15, 7, 17},
	{4096 * kb, 3, 15, 7, 18},
	{6144 * kb, 3, 16, 7, 18},
	{8192 * kb, 3, 16, 8, 19},
	{14336 * kb, 3, 16, 8, 20},
	{27548 * kb, 3, 16, 8, 21},
	{54278 * kb, 3, 16, 8, 22},
}
