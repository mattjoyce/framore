# framore

CLI tool for managing field recording sessions. Organises audio files into batches, runs GPU-accelerated bird species detection via [BirdNET](https://github.com/kahst/BirdNET-Analyzer), fetches weather data, and generates narrative reports with Ollama.

Built for a specific workflow: AudioMoth recordings on a NAS, processed via [Ductile](https://github.com/mattjoyce/ductile) for GPU job scheduling.

## Pipeline

```
framore new session1
framore add /Volumes/field_Recording/F3/Orig/20250921/*.WAV
framore add /Volumes/field_Recording/F3/Orig/20250921/*.JPG
framore config
framore start
```

Stages run in order:

| Stage | Where | What |
|-------|-------|------|
| **exif** | local | Extract GPS and timestamps from photos |
| **weather** | local | Fetch historical weather from Open-Meteo (cached) |
| **birdnet** | remote | Submit all audio to Ductile for GPU-accelerated BirdNET inference |
| **report** | remote | Generate a narrative summary via Ollama |

The BirdNET stage uses submit-all-then-poll: all audio files are submitted upfront, then polled for completion. This maximises GPU throughput when processing large sessions (72+ files).

## Commands

| Command | Description |
|---------|-------------|
| `framore new <name>` | Create a new batch YAML and set it as active |
| `framore use <batch.yaml>` | Set an existing batch file as active |
| `framore add <path>` | Add files to the active batch |
| `framore remove <path>` | Remove a file entry from the batch |
| `framore list` | Print all files in the active batch |
| `framore config` | Interactive wizard to configure the active batch |
| `framore status` | Show active batch and pipeline stage summary |
| `framore start` | Execute the pipeline against the active batch |
| `framore queue` | Show Ductile job queue status |
| `framore reset` | Clear the active batch setting |

## Install

```bash
go install github.com/mattjoyce/framore@latest
```

Or build from source:

```bash
git clone https://github.com/mattjoyce/framore.git
cd framore
go build -o bin/framore .
```

## Configuration

Global config lives at `~/.config/framore/config.toml`:

```toml
[defaults]
timezone = "Australia/Sydney"
default_lat = -34.0021
default_lon = 150.4987
birdnet_min_conf = 0.6

[paths]
processing_root = "/mnt/user/field_Recording"
allowed_paths = ["/Volumes/field_Recording", "/mnt/field_Recording"]

[services]
ductile_api_url = "http://192.168.20.4:8888"
ductile_token_env = "FRAMORE_DUCTILE_TOKEN"
ollama_url = "http://192.168.20.4:11434"
```

The Ductile API token is read from the environment variable named in `ductile_token_env` — no secrets in the config file.

## Requirements

- Go 1.23+
- [Ductile](https://github.com/mattjoyce/ductile) running on a GPU-equipped host with the birda plugin
- Ollama (for report generation)
- Audio files accessible via NAS mount

## License

[MIT](LICENSE)
