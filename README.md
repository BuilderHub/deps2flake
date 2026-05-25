# deps2flake

[![codecov](https://codecov.io/gh/BuilderHub/deps2flake/graph/badge.svg?token=3EMVGM8I0P)](https://codecov.io/gh/BuilderHub/deps2flake)

Generate a Nix flake from the dependency files already in your project.

`deps2flake` supports **Go** and **Python** projects behind the same CLI. Each
technology has its own generator and Nix builder:

| Technology | Dependency files | Nix stack |
|------------|------------------|-----------|
| Go | `go.mod`, `go.sum` → `nopher.lock.yaml` | [`nopher`](https://github.com/anthr76/nopher) `buildNopherGoApp` |
| Python | `pyproject.toml`, `uv.lock` | [`uv2nix`](https://github.com/pyproject-nix/uv2nix) + [pyproject.nix](https://github.com/pyproject-nix/pyproject.nix) |

The goal is to keep the CLI boring and let each language own its own generator.

## Go example

From a Go project:

```sh
deps2flake generate .
```

That writes:

```text
flake.nix
nopher.lock.yaml
```

To write everything somewhere else:

```sh
deps2flake generate . --out nix
```

That writes `nix/flake.nix` and `nix/nopher.lock.yaml`.

To include a container image package:

```sh
deps2flake generate . --container
```

Then build the default package with:

```sh
nix build .#default
```

## Python example

From a uv/pyproject project (with `pyproject.toml`; `uv.lock` is created if missing):

```sh
deps2flake generate . --tech python
```

That writes:

```text
flake.nix
uv.lock
```

Python-specific flags use the `--python.` prefix, for example:

```sh
deps2flake generate . --tech python \
  --python.interpreter pkgs.python312 \
  --python.script my-cli \
  --container
```

If a directory has both `go.mod` and `pyproject.toml`, auto-detection picks Go.
Use `--tech python` or `--tech go` explicitly.

## Development

```sh
make help
make dev
make all
```
