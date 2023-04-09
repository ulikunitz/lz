# TODO lz module

## updates
* Make a bTreeHash finder
* Make bTree also a match finder and implement the bTree functions in terms of
  bPath manipulations instead of recursive functions
* Write general sequencer that uses multiple match finders, is backward looking
  and supports also a repeater.
* We can now experiment using the general sequencer.

## v0.2.0

* Write documentation for each client and document defaults
* Package documentation should have table of supported sequencers
* We support go1.19 and above
* Write README.md
  - be explicit about no backward compatibility will be maintained; we
    will stay forever with v0
* Setup security policy
* Fuzz Sequencer and Decoder with go1.20
* Make v0.2.0
* Make repository public on github
