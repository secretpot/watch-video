# Develop watch-video

[Project README](../../README.md) · [简体中文](README.zh-CN.md)

## Build from source

In the source checkout, use the Go toolchain version in `go.mod` and install the required FFmpeg tools. Run `go test ./...` from the repository root. See the [installation guide](../setup/INSTALL.md) for platform requirements.

**macOS**

```sh
mkdir -p bin
CGO_ENABLED=0 go build -trimpath -o bin/watch-video .
```

**Windows PowerShell**

```powershell
New-Item -ItemType Directory -Force bin | Out-Null
$env:CGO_ENABLED = '0'
go build -trimpath -o bin/watch-video.exe .
```

## Contribute

Use [Issues](https://github.com/secretpot/watch-video/issues) to report a bug or propose a change. Include your OS, the relevant `doctor` output, and a public or synthetic reproduction.
