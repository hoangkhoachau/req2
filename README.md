# req2

A CLI tool for testing gRPC endpoints. It compiles your `.proto` files, generates a JSON request template, opens it in your `$EDITOR`, sends the request, and caches the input for next time.

## Install

```sh
go install github.com/hoangkhoachau/req2@latest
```

## Usage

```sh
req2 -a localhost:50051 -p ./api.proto Greeter/SayHello
```

1. Opens the JSON request template in `$EDITOR` (falls back to `vi`)
2. You fill in the fields and save
3. Sends the request and prints the response as JSON
4. Caches your input — next run pre-fills the editor with your last values

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--address` | `-a` | gRPC server address |
| `--proto` | `-p` | Proto file or directory (repeatable, default: `.`) |
| `--insecure` | `-i` | Disable TLS |
| `--repeat` | `-r` | Skip editor, reuse cached input |
| `--timeout` | `-t` | Request timeout, e.g. `30s`, `1m` (default: no timeout) |
| `--header` | `-H` | gRPC metadata header `key:value` (repeatable) |

### Piping input

```sh
echo '{"name": "world"}' | req2 -a localhost:50051 -p ./api.proto Greeter/SayHello
```

When stdin is not a terminal, the editor is skipped and stdin is used as the request body.

### Inspect

Print the JSON template for a method or message without sending a request:

```sh
req2 inspect -p ./api.proto Greeter/SayHello
req2 inspect -p ./api.proto HelloRequest
```

### Cache

```sh
req2 cache clear
```

## Shell Completion

Tab-completes `Service/Method` names from your proto files.

**Fish:**
```sh
req2 completion fish > ~/.config/fish/completions/req2.fish
```

**Zsh:**
```sh
echo 'source <(req2 completion zsh)' >> ~/.zshrc
```

**Bash:**
```sh
echo 'source <(req2 completion bash)' >> ~/.bashrc
```

The `-p` flag must be provided before pressing Tab:

```sh
req2 -p ./api.proto <Tab>   # lists available methods
```

## Config file

A default config is created at `~/.config/req2/config.yaml` on first run:

```yaml
address: ""
insecure: false
timeout: 30s
```

## Limitations

- Unary RPCs only — streaming is not supported
