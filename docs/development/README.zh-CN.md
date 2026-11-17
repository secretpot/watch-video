# 开发 watch-video

[English](README.md) · [项目首页](../../README.zh-CN.md)

## 从源码构建

获取所需版本的源码，使用 `go.mod` 指定的 Go 工具链，安装平台对应的 FFmpeg 工具。在仓库根目录运行 `go test ./...`，再构建可执行文件。[媒体工具安装](../setup/INSTALL.zh-CN.md)

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

## 反馈和贡献

通过 Issues 报告问题或提出改动建议。请附上操作系统、相关的 `doctor` 输出，并提供可公开访问的录播素材或合成复现素材。[GitHub Issues](https://github.com/secretpot/watch-video/issues)
