# helm-walk

**helm-walk** is a command-line utility to flatten and inspect deeply nested YAML files, such as Helm values or Kubernetes manifests. It prints each leaf value as a single line with its full key path, making it easy to search, diff, or audit configuration.

## Features

- Flattens nested YAML structures into key-value pairs.
- Supports custom entrypoints (`-e`) to start from any nested object by providing absolute object path.
- Handles arrays and prints indices in paths (e.g., `spec.containers[0].name`).
- Preserves multiline values and YAML-sensitive characters.
- Optionally includes or excludes empty values (`--all` / `--pure`).
- Output to stdout or file.

## Usage

```sh
helm-walk -f values.yaml
```

### Options

| Option                | Description                                         |
|-----------------------|-----------------------------------------------------|
| `-f, --file`          | Path to YAML file                                   |
| `-e, --entry`         | Entrypoint key (e.g., `alertmanager`)               |
| `-d, --depth`         | Limit output to a specific depth                    |
| `-A, --all`           | Include empty values (`""`, `{}`, `[]`)             |
| `-p, --pure`          | Only show non-empty values                          |
| `-o, --output`        | Write output to file                                |
| `-h, --help`          | Show help                                           |

### Example

Given a YAML file:

```yaml
alertmanager:
  enabled: false
  namespaceOverride: ""
  annotations:
    snooze: true
  apiVersion: v2
```

Run:

```sh
helm walk -f values.yaml -e alertmanager
```

Output:

```yaml
alertmanager.enabled: false
alertmanager.namespaceOverride: ""
alertmanager.annotations.snooze: true
alertmanager.apiVersion: v2
```

## Installation

Copy `main.go` to your Helm plugins directory or build as a standalone Go binary:

```sh
go build -o helm-walk main.go
```
