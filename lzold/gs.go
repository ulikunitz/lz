package lzold

import "fmt"

type MatchFinderConfig interface {
	NewMatchFinder() (mf MatchFinder, err error)
	ApplyDefaults()
	Verify() error
}

type MatchFinder interface {
	Add(pos uint32, x uint64)
	AppendMatchesAndAdd(m []uint32, pos uint32, x uint64) []uint32
	Adapt(delta uint32)

	// Resets the match finder and sets the pointer to the new data slice. The
	// pointer is used to ensure that length changes are available to the match
	// finders.
	Reset(pdata *[]byte)
}

type mfs []MatchFinder

func (s mfs) Add(pos uint32, x uint64) {
	for _, f := range s {
		f.Add(pos, x)
	}
}

func (s mfs) AppendMatchesAndAdd(m []uint32, pos uint32, x uint64) []uint32 {
	for _, f := range s {
		m = f.AppendMatchesAndAdd(m, pos, x)
	}
	return m
}

func (s mfs) Adapt(delta uint32) {
	for _, f := range s {
		f.Adapt(delta)
	}
}

func (s mfs) Reset(pdata *[]byte) {
	for _, f := range s {
		f.Reset(pdata)
	}
}

type GenericSequencerConfig struct {
	SBConfig
	MatchFinderConfigs []MatchFinderConfig
	CostEstimator      CostEstimator
}

func (cfg *GenericSequencerConfig) ApplyDefaults() {
	cfg.SBConfig.ApplyDefaults()
	if len(cfg.MatchFinderConfigs) == 0 {
		cfg.MatchFinderConfigs = []MatchFinderConfig{
			&HashConfig{
				InputLen: 3,
				HashBits: 18,
			},
		}
	}
	if cfg.CostEstimator == nil {
		cfg.CostEstimator = new(SimpleEstimator)
	}
}

func (cfg *GenericSequencerConfig) Verify() error {
	var err error
	if err = cfg.SBConfig.Verify(); err != nil {
		return err
	}
	if len(cfg.MatchFinderConfigs) == 0 {
		return fmt.Errorf("lz: MatchFinderConfigs is empty")
	}
	for i, c := range cfg.MatchFinderConfigs {
		err = c.Verify()
		if err != nil {
			return fmt.Errorf("lz: MatchFinderConfigs[%d] error %w",
				i, err)
		}
	}
	if cfg.CostEstimator == nil {
		return fmt.Errorf("lz: no cost estimator")
	}
	return nil
}

func (cfg *GenericSequencerConfig) NewSequencer() (s Sequencer, err error) {
	cfg.ApplyDefaults()
	if err = cfg.Verify(); err != nil {
		return nil, err
	}
	gs := new(genericSequencer)
	if err = gs.Init(*cfg); err != nil {
		return nil, err
	}
	panic("TODO")
}

type genericSequencer struct {
	SeqBuffer

	matchFinder   MatchFinder
	costEstimator CostEstimator
}

func (s *genericSequencer) Init(cfg GenericSequencerConfig) error {
	cfg.ApplyDefaults()
	var err error
	if err = cfg.Verify(); err != nil {
		return err
	}

	q := make(mfs, len(cfg.MatchFinderConfigs))
	for i, c := range cfg.MatchFinderConfigs {
		q[i], err = c.NewMatchFinder()
		if err != nil {
			return err
		}
	}
	if len(q) == 1 {
		s.matchFinder = q[0]
	} else {
		s.matchFinder = q
	}
	s.costEstimator = cfg.CostEstimator
	return nil
}

func (s *genericSequencer) Reset(data []byte) error {
	var err error
	if err = s.SeqBuffer.Reset(data); err != nil {
		return err
	}
	s.matchFinder.Reset(&s.SeqBuffer.data)
	s.costEstimator.Reset()
	return nil
}

func (s *genericSequencer) Shrink() {
	delta := uint32(s.SeqBuffer.shrink())
	s.matchFinder.Adapt(delta)
}

func (s *genericSequencer) hashSegment(a, b int) {
	/*
		if a < 0 {
			a = 0
		}
		c := len(s.data) - s.inputLen + 1
		if b > c {
			b = c
		}

		// Ensure that we can use _getLE64 all the time.
		_p := s.data[:b+7]

		for i := a; i < b; i++ {
			x := _getLE64(_p[i:])
			s.matchFinder.Add(uint32(i), x)
		}
	*/
}
