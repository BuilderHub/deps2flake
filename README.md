# deps2flake

Generate a Nix flake from the dependency files already in your project.

Right now `deps2flake` supports Go projects. It reads `go.mod`, asks
[`nopher`](https://github.com/anthr76/nopher) to create `nopher.lock.yaml`, and
then writes a `flake.nix` that builds the app with a default package. It can
also add a container package when you ask for one.

The goal is to keep the CLI boring and let each language own its own generator.
Go is first; other ecosystems can plug in behind the same scaffold interface.

## Example

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

## Development

```sh
make help
make dev
make all
```
