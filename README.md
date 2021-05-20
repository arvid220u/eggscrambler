# Eggscrambler

See `report.pdf` for a detailed summary of the system. Also, [here's a presentation](https://docs.google.com/presentation/d/122sLY_LXIb_H2V4cNmqFdX6s0l_g7-MTM48DuPij4Z4/edit?usp=sharing), including a demo at the last slide.

## Installation

Run:
```
git clone https://github.com/arvid220u/eggscrambler
```
(if you have access to [eggscrambler-raft](https://github.com/arvid220u/eggscrambler-raft), run instead `git clone --recursive git@github.com/arvid220u/eggscrambler`)

For testing:
```
cd anonbcast && RAFT=../raft-darwin20.2.0.so go test -v
```
(where `raft-darwin20.2.0.so` (for mac) can be replaced with another platform-specific version. the `RAFT` environment variable may be removed if you have access to `eggscrambler-raft`)

For running the application:
```
cd confessions && RAFT=../raft-darwin20.2.0.so go run confessions.go
```

## Overview

Key packages:
- `anonbcast`: providing the `anonbcast.Client` and `anonbcast.Server` that together implement the interface for executing the anonymous broadcasting protocol.
- `masseyomura`: an implementation of the Massey-Omura commutative encryption scheme.
- `raft`: an implementation of Raft, including configuration changes.
- `confessions`: an example application using `anonbcast`.


## Raft

Because the Raft code is a solution to the 6.824 labs, the staff has asked us to not publish it. Therefore, it is in a private submodule (following [this guide](https://www.taniarascia.com/git-submodules-private-content/)).
