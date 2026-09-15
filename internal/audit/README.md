## Профилирование памяти (инкремент 17)

### Что оптимизировано

- `FileStorage.saveFile`: `json.Marshal` с промежуточным `[]URLRecord` и `[]byte` заменён на потоковый `json.Encoder`, пишущий напрямую в файл.
- `FileStorage.Init`: `os.ReadFile` + `json.Unmarshal` заменены на потоковый `json.NewDecoder` с итерацией `decoder.More()`.
- `generateUUID`: самописная генерация через `crypto/rand` + `fmt.Sprintf` заменена на `github.com/google/uuid`.

### Команды

Снятие базового профиля (до оптимизаций):

```bash
go test -bench=. -benchmem -memprofile=profiles/base.pprof ./internal/repository
```

Снятие профиля после оптимизаций:
```bash
go test -bench=. -benchmem -memprofile=profiles/result.pprof ./internal/repository
```
Сравнение:
```bash
go tool pprof -top -diff_base=profiles/base.pprof profiles/result.pprof
```

Результат:
```
File: repository.test.exe
Build ID: D:\projects-my\practicum_2026\shortener\repository.test.exe2026-09-15 22:41:56.9228714 +0300 MSK
Type: alloc_space
Time: 2026-09-15 22:40:18 MSK
Showing nodes accounting for -13153.53kB, 37.10% of 35450.91kB total
      flat  flat%   sum%        cum   cum%
-3133.51kB  8.84%  8.84% -5240.39kB 14.78%  encoding/json.Marshal
-3131.77kB  8.83% 17.67% -3131.77kB  8.83%  os.readFileContents
-3112.05kB  8.78% 26.45% -3112.05kB  8.78%  reflect.growslice
-2704.33kB  7.63% 34.08% -8436.15kB 23.80%  github.com/b602op/shortener/internal/repository.(*FileStorage).Init
-2106.88kB  5.94% 40.02% -2106.88kB  5.94%  bytes.growSlice
-1046.99kB  2.95% 42.98% -6287.38kB 17.74%  github.com/b602op/shortener/internal/repository.(*FileStorage).saveFile
  544.67kB  1.54% 41.44%   544.67kB  1.54%  os.init.func1
  513.50kB  1.45% 39.99% -2072.87kB  5.85%  github.com/b602op/shortener/internal/repository.(*FileStorage).Insert
 -512.22kB  1.44% 41.44%  -512.22kB  1.44%  runtime.malg
  512.04kB  1.44% 39.99%   512.04kB  1.44%  syscall.UTF16FromString
  512.01kB  1.44% 38.55%   512.01kB  1.44%  encoding/json.(*decodeState).literalStore
  512.01kB  1.44% 37.10%   512.01kB  1.44%  github.com/b602op/shortener/internal/repository.TestStorage_Concurrent.func1
  512.01kB  1.44% 35.66%   512.01kB  1.44%  runtime.(*timers).addHeap
 -512.01kB  1.44% 37.10%  -512.01kB  1.44%  testing.fmtDuration
         0     0% 37.10% -2106.88kB  5.94%  bytes.(*Buffer).WriteString
         0     0% 37.10% -2106.88kB  5.94%  bytes.(*Buffer).grow
         0     0% 37.10%  2048.08kB  5.78%  encoding/json.(*Decoder).Decode
         0     0% 37.10% -4648.12kB 13.11%  encoding/json.(*decodeState).array
         0     0% 37.10%   512.01kB  1.44%  encoding/json.(*decodeState).object
         0     0% 37.10% -2600.04kB  7.33%  encoding/json.(*decodeState).unmarshal
         0     0% 37.10% -2600.04kB  7.33%  encoding/json.(*decodeState).value
         0     0% 37.10% -2106.88kB  5.94%  encoding/json.(*encodeState).marshal
         0     0% 37.10% -2106.88kB  5.94%  encoding/json.(*encodeState).reflectValue
         0     0% 37.10% -4648.12kB 13.11%  encoding/json.Unmarshal
         0     0% 37.10% -2106.88kB  5.94%  encoding/json.arrayEncoder.encode
         0     0% 37.10% -2106.88kB  5.94%  encoding/json.sliceEncoder.encode
         0     0% 37.10% -2106.88kB  5.94%  encoding/json.structEncoder.encode
         0     0% 37.10%    -3701kB 10.44%  github.com/b602op/shortener/internal/repository.(*FileStorage).BatchInsert
         0     0% 37.10% -9562.71kB 26.97%  github.com/b602op/shortener/internal/repository.BenchmarkFileStorage_BatchInsert
         0     0% 37.10% -4647.32kB 13.11%  github.com/b602op/shortener/internal/repository.BenchmarkFileStorage_Insert
         0     0% 37.10%   544.67kB  1.54%  os.(*File).Readdirnames
         0     0% 37.10%   544.67kB  1.54%  os.(*File).readdir
         0     0% 37.10%   512.04kB  1.44%  os.OpenFile
         0     0% 37.10% -3131.77kB  8.83%  os.ReadFile
         0     0% 37.10%  1056.71kB  2.98%  os.RemoveAll (inline)
         0     0% 37.10%   512.04kB  1.44%  os.openFileNolog
         0     0% 37.10%  1056.71kB  2.98%  os.removeAll
         0     0% 37.10%   544.67kB  1.54%  os.removeAllFrom
         0     0% 37.10% -3112.05kB  8.78%  reflect.Value.Grow
         0     0% 37.10% -3112.05kB  8.78%  reflect.Value.grow
         0     0% 37.10%   512.05kB  1.44%  regexp/syntax.(*parser).literal
         0     0% 37.10%   512.01kB  1.44%  runtime.(*scavengerState).sleep
         0     0% 37.10%   512.01kB  1.44%  runtime.(*timer).maybeAdd
         0     0% 37.10%   512.01kB  1.44%  runtime.(*timer).modify
         0     0% 37.10%   512.01kB  1.44%  runtime.(*timer).reset (inline)
         0     0% 37.10%   512.01kB  1.44%  runtime.bgscavenge
         0     0% 37.10%  -512.22kB  1.44%  runtime.newproc.func1
         0     0% 37.10%  -512.22kB  1.44%  runtime.newproc1
         0     0% 37.10%  -512.22kB  1.44%  runtime.systemstack
         0     0% 37.10%   544.67kB  1.54%  sync.(*Pool).Get
         0     0% 37.10%   512.04kB  1.44%  syscall.Open
         0     0% 37.10%   512.04kB  1.44%  syscall.UTF16PtrFromString (inline)
         0     0% 37.10% -13697.99kB 38.64%  testing.(*B).launch
         0     0% 37.10%   544.67kB  1.54%  testing.(*B).run1.func1
         0     0% 37.10% -13153.32kB 37.10%  testing.(*B).runN
         0     0% 37.10%  1056.71kB  2.98%  testing.(*B).runN.func1
         0     0% 37.10%  -512.01kB  1.44%  testing.(*T).report
         0     0% 37.10%  1056.71kB  2.98%  testing.(*common).Cleanup.func1
         0     0% 37.10%  1056.71kB  2.98%  testing.(*common).TempDir.func2
         0     0% 37.10%  1056.71kB  2.98%  testing.(*common).runCleanup
         0     0% 37.10%  1056.71kB  2.98%  testing.removeAll
         0     0% 37.10%  -512.01kB  1.44%  testing.tRunner
         0     0% 37.10%  -512.01kB  1.44%  testing.tRunner.func1
```

### Бенчмарки

| Метрика | До | После | Изменение |
|---------|-----|-------|-----------|
| `Insert` B/op | 72 460 | 50 029 | −31% |
| `Insert` allocs/op | 265 | 347 | +31% |
| `BatchInsert` B/op | 219 259 | 145 981 | −33% |
| `BatchInsert` allocs/op | 692 | 960 | +39% |
| `Select` B/op | 0 | 0 | без изменений |

Объём памяти упал на треть. Количество аллокаций выросло из-за `google/uuid`,
который делает больше мелких аллокаций при форматировании hex — но общий
объём выделяемой памяти снизился, что и было целью оптимизации.