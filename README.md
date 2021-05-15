# Eggscrambler

Key packages:
- `anonbcast`: providing the `anonbcast.Client` and `anonbcast.Server` that together implement the interface for executing the anonymous broadcasting protocol.
- `masseyomura`: an implementation of the Massey-Omura commutative encryption scheme.
- `raft`: an implementation of Raft, including configuration changes.
- `confessions`: an example application using `anonbcast`.

For testing:
```
cd anonbcast && go test -v
```

For running the application:
```
cd confessions && go run confessions.go
```
