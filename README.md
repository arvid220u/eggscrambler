# Eggscrambler

See `report.pdf` for a detailed summary of the system. Also, [here's a presentation](https://docs.google.com/presentation/d/122sLY_LXIb_H2V4cNmqFdX6s0l_g7-MTM48DuPij4Z4/edit?usp=sharing), including a demo at the last slide.

## Installation

Run:
```
git clone https://github.com/arvid220u/eggscrambler
```
(if you have access to [eggscrambler-raft](https://github.com/arvid220u/eggscrambler-raft), run instead `git clone --recursive git@github.com/arvid220u/eggscrambler`)

Then run:

```
cd eggscrambler && git submodule init && git submodule update go-libp2p-gorpc
```

For testing:
```
cd anonbcast && RAFT=../raft-darwin20.2.0.so go test -v
```
(where `raft-darwin20.2.0.so` (for mac) can be replaced with another platform-specific version. the `RAFT` environment variable may be removed if you have access to `eggscrambler-raft`)

For running the application:
```
cd confessions && RAFT=../raft-darwin20.2.0.so go run confessions.go
```

You might need to comment out lines 6 and 24 in `anonbcast/raft.go` if you don't have access to eggscrambler-raft. If your Go version is not the same as when we libified our Raft implementation, you may get an error `cannot load raft plugin ../raft-darwin20.2.0.so`.

## Overview

Key packages:
- `anonbcast`: providing the `anonbcast.Client` and `anonbcast.Server` that together implement the interface for executing the anonymous broadcasting protocol.
- `masseyomura`: an implementation of the Massey-Omura commutative encryption scheme.
- `raft`: an implementation of Raft, including configuration changes.
- `confessions`: an example application using `anonbcast`.


## Raft

Because the Raft code is a solution to the 6.824 labs, the staff has asked us to not publish it. Therefore, it is in a private submodule (following [this guide](https://www.taniarascia.com/git-submodules-private-content/)).
