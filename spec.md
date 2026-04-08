# framore — Field Recording Batch CLI

**Version:** 0.4 (draft)
**Language:** Go 1.22+
**Location:** `/Volumes/Projects/fram-harness/framore/`
**Global config:** `~/.config/framore/config.toml`

---

## Context

framore is the Mac-side CLI companion to the fram-harness pipeline running on the NAS (Unraid, 192.168.20.4).

The NAS does the heavy compute via Docker containers queued through Ductile:
- **birda** (birdnet-annotations) — GPU/TensorFlow, queued via Ductile webhook `POST /webhook/birda`
- **ollama** — LLM at `http://192.168.20.4:11434`, used by the **report** stage to write the session narrative
- **faster-whisper** — GPU/CUDA, *deferred: container not yet built*

framore CLI prepares batch YAML files, triggers stages, and monitors progress. It does not run GPU inference locally.

**v0.1 scope:** weather stage + birda (BirdNET) stage. All other stages are specced but not implemented.

---

## Go Modules

```
github.com/spf13/cobra            CLI framework — commands, flags, help text
gopkg.in/yaml.v3                  YAML batch file read/write
github.com/pelletier/go-toml/v2   TOML global config read/write
github.com/go-audio/wav           WAV metadata: bit depth, sample rate, channels, duration
github.com/rwcarlsen/goexif/exif  EXIF from images: GPS, datetime, device
github.com/charmbracelet/huh      Interactive config wizard (forms, selects, confirms)
```

stdlib only for: HTTP, HMAC, time, filepath, os, json.

Audio inspection is **WAV only** for v0.1. Do not add FLAC/MP3 support until asked.

---

## Tooling

### Required tools

```bash
go install golang.org/x/tools/cmd/goimports@latest
go install github.com/securego/gosec/v2/cmd/gosec@latest
brew install golangci-lint
```

`gofmt` ships with Go. Run all three before every commit.

### golangci-lint config

Create `.golangci.yml` at the project root:

```yaml
linters:
  enable:
    - gofmt
    - goimports
    - gosec
    - errcheck
    - govet
    - staticcheck
    - unused

linters-settings:
  goimports:
    local-prefixes: github.com/mattjoyce/framore
```

Run with:
```bash
golangci-lint run ./...
```

Fix all lint errors before committing. Do not add `//nolint` comments without a comment explaining why.

### gosec

```bash
gosec ./...
```

Pay attention to G107 (URL from variable — expected for our HTTP calls, but document the suppression), G401/G501 (weak crypto — never use MD5/SHA1 for HMAC; we use SHA256). Any other gosec finding must be fixed, not suppressed.

---

## Commit Message Convention

framore uses **Conventional Commits** so the changelog can be generated automatically.

```
<type>(<scope>): <short description>

[optional body]
```

**Types:**
- `feat` — new feature or command
- `fix` — bug fix
- `test` — adding or fixing tests
- `refactor` — code change with no behaviour change
- `chore` — tooling, deps, config (no production code change)
- `docs` — spec, README, comments only

**Scopes** (use the component name):
- `add`, `config`, `start`, `use`, `new` — CLI commands
- `batch`, `pipeline`, `stages` — internal packages
- `birdnet`, `weather`, `exif`, `report` — individual stages
- `ductile`, `ollama` — service clients

**Examples:**
```
feat(weather): implement open-meteo archive fetch with local cache
fix(birdnet): clamp ISO week > 48 to 48 for BirdNET scale
test(batch): add validate_paths tests for allowed_paths guard
chore: add golangci-lint config and Makefile targets
```

Keep the subject line under 72 characters. Body is optional but useful for non-obvious decisions.

---

## Testing

### Philosophy

- Test behaviour, not implementation.
- No mocking of the filesystem — use `t.TempDir()` and real files.
- No mocking of the batch YAML loader — write a real YAML, load it.
- Mock HTTP only at the transport layer (never mock your own functions).
- Each test file lives next to the file it tests: `batch_test.go` beside `batch.go`.

### What to test

| Package | Test focus |
|---|---|
| `internal/batch` | load/save round-trip, validate_paths rejects bad paths, inspect correctly reads WAV metadata |
| `internal/config` | load defaults when no file, save/reload preserves values |
| `internal/pipeline` | runner calls stages in order, skips disabled stages, writes Results correctly |
| `internal/stages/weather` | cache hit skips API call, cache miss fetches and stores, handles missing GPS gracefully |
| `internal/stages/birdnet` | week clamping (week 50 → 48), path translation (Mac → NAS), payload JSON shape |
| `internal/ductile` | HMAC signature matches reference vector, POST sends correct headers |

### HTTP testing pattern

Use `net/http/httptest.NewServer` to stand up a real local server. Never import `testify/mock` or similar.

```go
// Example: testing the ductile client
func TestDuctilePost(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // assert headers, body shape
        w.WriteHeader(http.StatusOK)
        w.Write([]byte(`{"status":"ok"}`))
    }))
    defer srv.Close()

    client := ductile.NewClient(srv.URL, "test-secret")
    err := client.Post(context.Background(), payload)
    // assert err == nil
}
```

### WAV test fixture

Commit a minimal valid WAV file at `testdata/mono_24bit_48k.wav` for use in inspect tests. Generate it once with ffmpeg; do not regenerate in tests.

---

## Global Config

Location: `~/.config/framore/config.toml`

If the file does not exist, framore creates it with defaults on first run.

```toml
current_batch = ""   # absolute path; empty until framore use/new is called

[defaults]
timezone         = "Australia/Sydney"
default_lat      = -34.0021    # fallback GPS if no EXIF found in any photo
default_lon      = 150.4987
birdnet_min_conf = 0.6
birdnet_threads  = 4

[paths]
# Root path on the NAS where files are stored.
processing_root = "/mnt/user/field_Recording"
# Files added to a batch must be under one of these Mac-side prefixes.
# Path translation replaces the matching prefix with processing_root.
allowed_paths = [
    "/Volumes/field_Recording",
    "/mnt/field_Recording",
]

[services]
ductile_api_url    = "http://192.168.20.4:8888"
ductile_token_env  = "FRAMORE_DUCTILE_TOKEN"     # name of env var holding Bearer token
ollama_url         = "http://192.168.20.4:11434"

[weather]
cache_dir          = "~/.cache/framore/weather"
cache_max_age_days = 30
timeout_seconds    = 30

[output]
log_level = "info"   # debug | info | warn | error
```

**No secrets in config.** `ductile_token_env` is the *name* of an environment variable, not the token itself. The binary reads `os.Getenv(cfg.Services.DuctileTokenEnv)` at runtime.

---

## Batch File

A batch YAML is the unit of work. It lives wherever the user puts it and is referenced by absolute path in global config.

```yaml
# my-session.yaml
session_dir:  /Volumes/field_Recording/F3/Orig/260329-NationalPark-ForestPath-Calala
session_date: 2026-03-29   # YYYY-MM-DD, required. Used to derive BirdNET week.
                            # Set via 'framore config', not inferred from folder name.

stages:
  exif:       true    # always; builds timestamp→GPS map from photos
  weather:    true    # v0.1
  birdnet:    true    # v0.1
  transcribe: false   # deferred: faster-whisper container not yet built
  report:     true    # always; merges results, writes narrative via Ollama

birdnet:
  min_conf: 0.6
  threads:  4

weather:
  timezone: Australia/Sydney

pipeline:
  default_lat: -34.0021   # used if no EXIF GPS is found in any photo
  default_lon: 150.4987

files: []   # populated by framore add; do not edit by hand
```

The `files` list is managed by framore. Do not define it manually — it is written by `framore add` and read by `framore start`.

---

## Commands

### `framore new <name>`

Create a new batch YAML from the default template **and** set it as the active batch.

```
framore new fieldtrip-26apr
→ Created:      ./fieldtrip-26apr.yaml
→ Active batch: /absolute/path/fieldtrip-26apr.yaml
```

Template values come from `[defaults]` in global config. Opens in `$EDITOR` unless `--no-edit` flag is passed.

**Implementation note:** write the template, then call the same `setActiveBatch()` helper used by `framore use`.

---

### `framore use <batch.yaml>`

Set an existing batch file as active. Saves its absolute path to global config under `current_batch`. If the file does not exist, create it from template first (same as `new`).

```
framore use ./my-session.yaml
→ Active batch: /absolute/path/my-session.yaml

framore use ./new-trip.yaml        # file doesn't exist → created
→ Created:      ./new-trip.yaml
→ Active batch: /absolute/path/new-trip.yaml
```

**Implementation note:** always resolve to absolute path with `filepath.Abs` before storing.

---

### `framore add <path> [path...]`

Add files to the active batch. Accepts a single file, a glob (`*.WAV`), or a directory (recurse for supported formats).

**Supported formats (v0.1):** `.WAV`, `.wav`, `.jpg`, `.jpeg`, `.JPG`, `.JPEG`, `.png`

**Path guard:** resolve each file to an absolute path, then check it is under one of the `allowed_paths` prefixes. Reject with a clear message if not:
```
✗ /Users/matt/Downloads/test.wav — not under an allowed path
  Add a path mapping in ~/.config/framore/config.toml → [paths] allowed_paths
```

**On success**, inspect each file and append to `files:` in the batch YAML:

```yaml
files:
  - path: /Volumes/field_Recording/F3/Orig/.../221053_0001.WAV
    type: audio
    added: 2026-04-06T21:00:00Z
    meta:
      duration_seconds: 182.4
      bit_depth: 24
      sample_rate: 48000
      channels: 2
      size_bytes: 52428800
  - path: /Volumes/field_Recording/F3/Orig/.../IMG_4521.jpg
    type: image
    added: 2026-04-06T21:00:00Z
    meta:
      datetime: 2026-04-06T07:14:22Z
      lat: -34.0021
      lon: 150.4987
      altitude_m: 42.0
      device: Pixel 8
```

Files already present in `files:` are skipped (print a short notice). Unsupported formats are skipped with a warning.

**Implementation note:** read the existing batch YAML, append new entries, write it back. Use `gopkg.in/yaml.v3` with a round-trip-safe struct — do not unmarshal/re-marshal the whole file naively or comments will be lost. Read the file as a `yaml.Node`, manipulate the `files` sequence node directly.

---

### `framore remove <path>`

Remove a file entry from the batch by path. Does not touch the file on disk.

---

### `framore list`

Print all files in the active batch with their metadata summary. Plain text table is fine.

---

### `framore config`

Interactive wizard to set pipeline stage options in the active batch YAML. Uses `charmbracelet/huh` forms. Walks stages in order; deferred stages are shown as skipped.

```
── Session ────────────────────────────────────
  Recording date (YYYY-MM-DD): 2026-03-29
  (sets BirdNET week; not inferred from folder name)

── EXIF ───────────────────────────────────────
  (always enabled)
  Default GPS if no photos: -34.0021, 150.4987

── Weather ────────────────────────────────────
  Enable weather lookup? [Y/n]
  Timezone: [Australia/Sydney]

── BirdNET ────────────────────────────────────
  Enable BirdNET detection? [Y/n]
  Min confidence (0.0–1.0): [0.6]
  Threads: [4]

── Transcription ──────────────────────────────
  [deferred — faster-whisper container not yet available]

── Report ─────────────────────────────────────
  (always generated — uses Ollama at http://192.168.20.4:11434)

Save changes to ./my-session.yaml? [Y/n]
```

---

### `framore status`

Show active batch and pipeline stage summary.

```
Active batch: /Volumes/.../my-session.yaml
Session dir:  /Volumes/.../260329-NationalPark-ForestPath-Calala
Session date: 2026-03-29  (week 13 → BirdNET week 13)

Files: 9 audio, 3 images

Stages:
  [x] exif       local       GPS fallback: -34.0021, 150.4987
  [x] weather    local       timezone=Australia/Sydney
  [x] birdnet    → ductile   min_conf=0.6  threads=4
  [-] transcribe deferred
  [x] report     → ollama    http://192.168.20.4:11434
```

---

### `framore start [batch.yaml]`

Execute the pipeline against the active batch (or the named file).

```
framore start
framore start ./my-session.yaml
framore start --stage birdnet        # one stage only
framore start --dry-run              # print plan, don't execute
framore start --verbose              # per-file progress lines
```

**Execution order (v0.1):**

```
1. exif     local    parse EXIF from image files → build []PhotoGPS
2. weather  local    open-meteo API → WeatherResult (cached)
3. birdnet  remote   submit-all-then-poll via Ductile REST API → BirdNetFileResult per file + SessionBirdNetResult
4. report   remote   POST to Ollama → writes session_report.md
```

Stages that are disabled in the batch (`stages.birdnet: false`) are skipped silently unless `--verbose`.

**Implementation note:** the runner iterates `Registry`, calls `stage.Enabled(batch)`, then `stage.Run(ctx, batch, results)`. It prints `[stage name] starting…` and `[stage name] done` (or error) to stdout. Errors from a stage are logged but do not abort subsequent stages unless the stage explicitly returns a sentinel that the runner checks.

---

## Stage: EXIF

**Purpose:** parse GPS and datetime from `.jpg`/`.jpeg`/`.png` files in the batch. Produces a `[]PhotoGPS` written to results under key `"exif"/"session"`.

```go
type PhotoGPS struct {
    Path     string
    Time     time.Time
    Lat, Lon float64
    Altitude float64
}
```

**GPS lookup for audio files:** given an audio file's datetime (derived from its `meta.added` timestamp — or for F3 files, a future parser can use filename), find the `PhotoGPS` entry with the nearest timestamp. If no photos have GPS, use `pipeline.default_lat` / `pipeline.default_lon` from the batch (or global config fallback).

**Implementation note:** sort `[]PhotoGPS` by time, then for each audio file do a linear scan for minimum `|audio.time - photo.time|`. This is simple and correct for session sizes (< 100 files).

---

## Stage: Weather

**Purpose:** fetch historical weather from open-meteo for the session location and date.

**API:** `https://archive-api.open-meteo.com/v1/archive` — free, no API key.

**Request parameters:**
```
latitude      <lat>
longitude     <lon>
start_date    <session_date>
end_date      <session_date>
hourly        temperature_2m,relative_humidity_2m,precipitation,wind_speed_10m,weather_code,cloud_cover,pressure_msl
daily         sunrise,sunset
timezone      <batch.weather.timezone>
```

**Cache:** before fetching, check `~/.cache/framore/weather/<lat>_<lon>_<date>.json`. If the file exists and is younger than `cache_max_age_days`, load from cache. On fetch, write the raw response JSON to the cache file.

**Result stored in Results as:**
```go
type WeatherResult struct {
    Date        string
    Hourly      []HourlyWeather  // one entry per hour
    Sunrise     string
    Sunset      string
}
```

Store under `results.Set("weather", "session", weatherResult)`.

**Error handling:** if the API call fails, log the error and continue — weather is non-blocking. The report stage handles absent weather gracefully.

---

## Stage: BirdNET (birda)

**Purpose:** run bird species detection on each audio file via the Ductile REST API.

**Week derivation:** parse `session_date` from the batch as a `time.Time`. Compute ISO week number with `t.ISOWeek()`. Clamp to 1–48: if week > 48, use 48.

```go
_, week := sessionDate.ISOWeek()
if week > 48 {
    week = 48
}
```

**GPS resolution priority:**
1. EXIF centroid from photos (if exif stage ran and found GPS)
2. Plus code from `pipeline.plus_code` in the batch
3. `pipeline.default_lat` / `pipeline.default_lon` from the batch

**Path translation:** before building the payload, translate the Mac path to the NAS path using `batch.CheckAllowedPath()`. This replaces the first matching `allowed_paths` prefix with `processing_root`:
```
/Volumes/field_Recording/F3/Orig/file.WAV
→ /mnt/user/field_Recording/F3/Orig/file.WAV
```

### Execution model: submit-all-then-poll

The stage operates in two phases to maximise GPU throughput:

**Phase 1 — Submit all:** iterate every audio file in the batch and call `POST /plugin/birda/handle` via the Ductile REST API. Each successful submit returns a job ID. Collect all job IDs into a `[]pendingJob` slice. Submit errors for individual files are logged and skipped — they do not abort the batch. Non-audio files are skipped silently.

**Phase 2 — Poll all:** loop over remaining pending jobs, calling `GET /job/{id}` for each. Jobs still `"queued"` or `"running"` stay in the pending list. Jobs that reach `"succeeded"` have their result parsed into `BirdNetFileResult`. Jobs that reach `"failed"`, `"dead"`, or `"timed_out"` are logged and dropped. Between poll rounds, sleep 3 seconds. The loop exits when no jobs remain. Context cancellation is checked at the start of each round and during the sleep.

This replaces the earlier sequential submit→wait→submit→wait pattern. Since Ductile serialises GPU access internally, submitting all jobs upfront is safe and lets the queue fill immediately.

**Ductile REST API:**

Authentication: Bearer token via `Authorization: Bearer <token>` header.
Token source: `os.Getenv(cfg.Services.DuctileTokenEnv)`.

Submit: `POST {ductile_api_url}/plugin/birda/handle`

```json
{
  "payload": {
    "wav_path": "/mnt/user/field_Recording/F3/Orig/.../221053_0001.WAV",
    "lat": -34.0021,
    "lon": 150.4987,
    "min_conf": 0.6,
    "week": 13
  }
}
```

Submit response (202 Accepted):
```json
{
  "job_id": "abc-123",
  "status": "queued",
  "plugin": "birda",
  "command": "handle"
}
```

Poll: `GET {ductile_api_url}/job/{job_id}`

Job response (on success):
```json
{
  "job_id": "abc-123",
  "status": "succeeded",
  "result": {
    "output_path": "/mnt/.../birdnet_output/221053_0001.csv",
    "detections": [...],
    "detection_count": 12,
    "duration_s": 182.4,
    "realtime_factor": 4.5
  }
}
```

**Session-level unification:** after all polling completes, per-file detections are merged into a `SessionBirdNetResult` containing a deduplicated species summary with max confidence, total detections, and file counts per species.

**Results storage:**
- Per-file: `results.Set("birdnet", file.Path, birdNetFileResult)`
- Session: `results.Set("birdnet", "session", sessionBirdNetResult)`

**Elapsed timer:** total wall-clock time for the birdnet stage is printed on the final summary line.

**Error handling:** submit and poll errors for individual files are logged but do not abort the batch. The session result reflects only successfully completed files.

---

## Stage: Transcribe

**Purpose:** transcribe spoken field notes from the first and last N seconds of each WAV file using the faster-whisper REST API running on the NAS.

**Service:** `http://192.168.20.4:8765` (configurable via `config.toml` → `[services] whisper_url`). Direct HTTP — no Ductile queue. The whisper container is lightweight enough to call sequentially.

**Batch config:**

```yaml
transcribe:
  duration_seconds: 60   # transcribe first/last N seconds of each file
  language: ""           # blank = auto-detect; or "en", "de", etc.
```

**Execution model: sequential HTTP POST per file**

For each audio file in the batch:

1. Translate the Mac path to NAS path via `CheckAllowedPath()`
2. POST to `{whisper_url}/transcribe` with JSON payload
3. Parse the response and store the result

This is simpler than BirdNET — no job queue, no polling. Each file blocks until the whisper service returns.

**Payload (POST `/transcribe`):**

```json
{
  "wav_path": "/mnt/user/field_Recording/F3/Orig/.../221053_0001.WAV",
  "duration_seconds": 60,
  "language": ""
}
```

**Response (200 OK):**

```json
{
  "transcripts": [
    {
      "segment": "start",
      "text": "Walking along the creek trail, light rain, about 7am",
      "language": "en",
      "language_probability": 0.98
    },
    {
      "segment": "end",
      "text": "",
      "language": "",
      "language_probability": 0.0
    }
  ]
}
```

**Result type:**

```go
type TranscriptSegment struct {
    Segment             string  `json:"segment"`
    Text                string  `json:"text"`
    Language            string  `json:"language"`
    LanguageProbability float64 `json:"language_probability"`
}

type TranscribeFileResult struct {
    FilePath    string              `json:"file_path"`
    Transcripts []TranscriptSegment `json:"transcripts"`
}
```

**Results storage:**
- Per-file: `results.Set("transcribe", file.Path, transcribeFileResult)`

**Error handling:** if the whisper service is unreachable or returns an error for a file, log the error and skip to the next file. The stage does not abort the pipeline.

---

## Stage: Report

**Purpose:** merge all stage results and write `session_report.md` + `session.json` into the session folder. Uses Ollama to write the narrative section.

Deferred in detail — implement after weather and birdnet are stable.

---

## Stage Interface

Every stage implements:

```go
// internal/pipeline/stage.go

type Stage interface {
    Name() string
    Enabled(batch *batch.Batch) bool
    Run(ctx context.Context, b *batch.Batch, results *Results) error
}
```

The registry is a plain slice in `internal/pipeline/runner.go`. Execution order = slice order.

```go
var Registry = []Stage{
    &stages.EXIF{},
    &stages.Weather{},
    &stages.BirdNet{},
    &stages.Transcribe{},
    &stages.Report{},
}
```

**To add a new stage (complete checklist):**
1. Create `internal/stages/mystage.go` with a struct implementing `Stage`
2. Add the struct to `Registry` in `runner.go` at the right position
3. Add its config fields to `batch.StageConfig` in `internal/batch/batch.go`
4. Add its YAML block to the batch template in `internal/batch/template.go`
5. Add its wizard section to `cmd/config.go`
6. Add its display row to `cmd/status.go`
7. Write tests in `internal/stages/mystage_test.go`

---

## Results

```go
// internal/pipeline/stage.go

type Results struct {
    mu   sync.Mutex
    data map[string]map[string]any
}

// stage = "birdnet", "weather", etc.
// key   = absolute file path, or "session" for session-wide data
func (r *Results) Set(stage, key string, val any) { ... }
func (r *Results) Get(stage, key string) (any, bool) { ... }
func (r *Results) AllForStage(stage string) map[string]any { ... }
```

The mutex is required even though v0.1 runs stages sequentially — future stages may run concurrently, and the pattern should be safe from the start.

---

## Project Structure

```
framore/
  cmd/
    root.go         # cobra setup; loads global config; sets up logger
    use.go          # framore use
    new.go          # framore new
    add.go          # framore add
    remove.go       # framore remove
    list.go         # framore list
    config.go       # framore config (huh wizard)
    status.go       # framore status
    start.go        # framore start
    reset.go        # framore reset
  internal/
    batch/
      batch.go      # Batch struct, YAML load/save, StageConfig
      inspect.go    # inspectWAV() and inspectImage() — returns FileMeta
      validate.go   # CheckAllowedPath(path, cfg) — returns translated NAS path or error
      template.go   # defaultBatchYAML(cfg) — returns template string
    pipeline/
      stage.go      # Stage interface, Results type
      runner.go     # Run(ctx, batch, cfg, stages) — ordered execution
    stages/
      exif.go       # EXIF stage
      weather.go    # Weather stage
      birdnet.go    # BirdNET stage
      report.go     # Report stage (stub for now)
    ductile/
      client.go     # NewClient, Submit, GetJob, WaitForJob — Bearer token auth
    ollama/
      client.go     # NewClient, Generate — handles HTTP POST to /api/generate
    config/
      config.go     # Load(), Save() for ~/.config/framore/config.toml
  testdata/
    mono_24bit_48k.wav    # minimal WAV fixture for inspect tests
  main.go
  go.mod
  go.sum
  .golangci.yml
  Makefile
```

### Makefile targets

```makefile
.PHONY: build test lint sec fmt

build:
	go build -o bin/framore ./...

test:
	go test ./...

lint:
	golangci-lint run ./...

sec:
	gosec ./...

fmt:
	gofmt -w .
	goimports -w .

check: fmt lint sec test
```

Run `make check` before every commit.

---

## Deferred (not v0.1)

| Item | Notes |
|---|---|
| `transcribe` stage | Implemented — calls faster-whisper REST API directly |
| `spectrogram` stage | melspec-to-video; low priority |
| FLAC/MP3 inspection | WAV only for now |
| Parallel stage execution | GPU stages serialised by Ductile anyway |
| Windows support | Mac/Linux only |
