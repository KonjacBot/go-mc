# Benchmarks

`baseline.txt` is the read path before the buffering/zlib/limit changes,
`phase1.txt` after them. Both were recorded on the same machine with:

```bash
go test -run '^$' -bench 'BenchmarkUnpack|BenchmarkStringReadFrom|BenchmarkConnReadPacket' \
  -benchtime 300ms -count 6 ./net/packet ./net > bench/new.txt
benchstat bench/phase1.txt bench/new.txt
```

`BenchmarkConnReadPacket` reads a mixed, encrypted and compressed packet
stream through `Conn` and reports `reads/packet`: how many `Read` calls reach
the socket per packet (each one is a syscall on a real connection).
