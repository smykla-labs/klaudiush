# Development Environment Setup

## Prerequisites

This project uses [mise](https://mise.jdx.dev/) for managing development tool versions.

### Install mise

```bash
# macOS
brew install mise

# Linux
curl https://mise.run | sh

# Windows (WSL)
curl https://mise.run | sh
```

### Activate mise in your shell

Add this to your shell configuration (`~/.bashrc`, `~/.zshrc`, etc.):

```bash
eval "$(mise activate bash)"  # or zsh, fish, etc.
```

Or for the current session:

```bash
eval "$(mise activate bash --shims)"
```

## Quick Start

1. Install project tools:

```bash
mise install
```

This will install:

- Go 1.25.4
- golangci-lint 2.6.2

2. Verify installation:

```bash
mise list
```

3. Run development tasks:

```bash
task          # Show available tasks
task build    # Build the binary
task test     # Run tests
task lint     # Run linters
```

## Tool Versions

Tool versions are pinned in `.mise.toml`:

```toml
[tools]
go = "1.25.4"              # Latest stable Go as of 2025-11-23
golangci-lint = "2.6.2"    # Latest golangci-lint
```

To update versions:

1. Check available versions:

```bash
mise ls-remote go
mise ls-remote golangci-lint
```

2. Update `.mise.toml` with new versions
3. Run `mise install` to apply changes

## Benefits of mise

- **Version pinning**: Ensures all developers use the same tool versions
- **Automatic activation**: Tools are available when you `cd` into the project
- **Per-project isolation**: Different projects can use different versions
- **Fast**: Tools are downloaded and cached locally
- **No system pollution**: Tools are installed in `~/.local/share/mise`

## Without mise

If you prefer not to use mise, you can install tools manually:

```bash
# Install Go 1.25.4
# See https://go.dev/dl/

# Install golangci-lint 2.6.2
go install github.com/golangci/golangci-lint/cmd/golangci-lint@v2.6.2
```

Then update `Taskfile.yaml` to remove `mise exec --` prefixes from commands.
