{
  description = "deps2flake flake";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
    nopher.url = "github:anthr76/nopher";
  };

  outputs = { nixpkgs, flake-utils, nopher, ... }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        nopherLib = nopher.lib.${system};

        deps2flake = nopherLib.buildNopherGoApp {
          pname = "deps2flake";
          version = "0.1.0";
          src = ./.;
          modules = ./nopher.lock.yaml;
          subPackages = [ "./cmd/deps2flake" ];

          meta = {
            description = "Generate clean Nix flakes from existing dependency files";
            homepage = "https://github.com/BuilderHub/deps2flake";
            license = pkgs.lib.licenses.mit;
            mainProgram = "deps2flake";
          };
        };
      in
      {
        packages = {
          default = deps2flake;
          deps2flake = deps2flake;
        };

        apps.default = {
          type = "app";
          program = "${deps2flake}/bin/deps2flake";
        };

        devShells.default = pkgs.mkShell {
          packages = [
            pkgs.go
            pkgs.gopls
            pkgs.gotools
            pkgs.go-tools
            pkgs.golangci-lint
            pkgs.delve
            nopher.packages.${system}.default
          ];
        };
      });
}
