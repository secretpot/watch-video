package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const darwinHomebrewBin = "/opt/homebrew/bin"

type darwinBrewSource struct {
	name, api, bottles string
}

func setupMediaTools(ctx context.Context, mode setupMirror, _ string, progress io.Writer) (toolset, error) {
	if runtime.GOARCH != "arm64" {
		return toolset{}, errors.New("setup on macOS requires a native Apple Silicon build")
	}
	version, err := exec.CommandContext(ctx, "/usr/bin/sw_vers", "-productVersion").Output()
	if err != nil {
		return toolset{}, fmt.Errorf("determine macOS version: %w", err)
	}
	release := strings.TrimSpace(string(version))
	major, _, _ := strings.Cut(release, ".")
	number, err := strconv.Atoi(major)
	if err != nil || number < 15 {
		return toolset{}, fmt.Errorf("setup requires macOS 15 or later (found %q)", release)
	}

	dirs := append([]string{darwinHomebrewBin}, filepath.SplitList(os.Getenv("PATH"))...)
	for _, dir := range dirs {
		if !filepath.IsAbs(dir) {
			continue
		}
		tools := toolset{dir: dir}
		if _, err := missingDarwinFormulae(ctx, tools); err == nil {
			return tools, nil
		}
		if err := ctx.Err(); err != nil {
			return toolset{}, err
		}
	}

	brew, err := (toolset{dir: darwinHomebrewBin}).path("brew")
	if err != nil {
		return toolset{}, fmt.Errorf("find Homebrew in %s or PATH: %w\nInstall Homebrew from https://brew.sh/ or https://mirrors.tuna.tsinghua.edu.cn/help/homebrew/, then rerun setup", darwinHomebrewBin, err)
	}
	prefix, err := exec.CommandContext(ctx, brew, "--prefix").Output()
	if err != nil {
		return toolset{}, fmt.Errorf("read the Homebrew prefix from %s: %w", brew, err)
	}
	brewPrefix := strings.TrimSpace(string(prefix))
	if !filepath.IsAbs(brewPrefix) {
		return toolset{}, fmt.Errorf("Homebrew returned an invalid prefix %q", brewPrefix)
	}
	tools := toolset{dir: filepath.Join(brewPrefix, "bin")}
	missing, validationErr := missingDarwinFormulae(ctx, tools)
	if validationErr == nil {
		return tools, nil
	}

	var attempts []error
	for _, source := range darwinBrewSources(mode) {
		if err := ctx.Err(); err != nil {
			return toolset{}, err
		}
		fmt.Fprintf(progress, "Installing %s with Homebrew (%s)\n", strings.Join(missing, ", "), source.name)
		args := append([]string{"install"}, missing...)
		command := exec.CommandContext(ctx, brew, args...)
		command.Env = append(os.Environ(),
			"HOMEBREW_NO_AUTO_UPDATE=1",
			"HOMEBREW_ARTIFACT_DOMAIN=",
			"HOMEBREW_API_DOMAIN="+source.api,
			"HOMEBREW_BOTTLE_DOMAIN="+source.bottles,
		)
		command.Stdout = progress
		command.Stderr = progress
		command.WaitDelay = 2 * time.Second
		if err := command.Run(); err != nil {
			if err := ctx.Err(); err != nil {
				return toolset{}, err
			}
			attempt := fmt.Errorf("Homebrew (%s): %w", source.name, err)
			fmt.Fprintln(progress, attempt)
			attempts = append(attempts, attempt)
			missing, validationErr = missingDarwinFormulae(ctx, tools)
			if validationErr == nil {
				return tools, nil
			}
			continue
		}
		if _, err := missingDarwinFormulae(ctx, tools); err != nil {
			return toolset{}, fmt.Errorf("validate installed media tools in %s: %w", tools.dir, err)
		}
		return tools, nil
	}
	return toolset{}, fmt.Errorf("install media tools in %s: %w", tools.dir, errors.Join(append(attempts, validationErr)...))
}

func missingDarwinFormulae(ctx context.Context, tools toolset) ([]string, error) {
	var missing []string
	var problems []error
	for _, requirement := range []struct {
		formula string
		tools   []string
	}{
		{"ffmpeg", []string{"ffmpeg", "ffprobe"}},
		{"whisper.cpp", []string{"whisper-cli"}},
	} {
		for _, name := range requirement.tools {
			// All three tools must work independently of the host's PATH.
			_, err := exec.LookPath(filepath.Join(tools.dir, name))
			if err == nil {
				_, err = tools.version(ctx, name)
			}
			if err != nil {
				missing = append(missing, requirement.formula)
				problems = append(problems, fmt.Errorf("%s: %w", name, err))
				break
			}
		}
	}
	return missing, errors.Join(problems...)
}

func darwinBrewSources(mode setupMirror) []darwinBrewSource {
	official := darwinBrewSource{"official", "https://formulae.brew.sh/api", "https://ghcr.io/v2/homebrew/core"}
	tuna := "https://mirrors.tuna.tsinghua.edu.cn/homebrew-bottles"
	ustc := "https://mirrors.ustc.edu.cn/homebrew-bottles"
	china := []darwinBrewSource{{"Tsinghua TUNA", tuna + "/api", tuna}, {"USTC", ustc + "/api", ustc}}
	switch mode {
	case "official":
		return []darwinBrewSource{official}
	case "cn":
		return append(china, official)
	default:
		return append([]darwinBrewSource{official}, china...)
	}
}
