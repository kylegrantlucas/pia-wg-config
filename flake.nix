{
  description = "pia-wg-config dev shell";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";

  outputs = { self, nixpkgs }:
    let
      systems = [ "aarch64-darwin" "x86_64-darwin" "x86_64-linux" "aarch64-linux" ];
      forAll = f: nixpkgs.lib.genAttrs systems (s: f nixpkgs.legacyPackages.${s});
    in {
      devShells = forAll (pkgs: {
        default = pkgs.mkShell {
          packages = [ pkgs.go_1_27 pkgs.gopls pkgs.golangci-lint ];
          # Use the Nix-provided Go; don't auto-download the go.mod toolchain.
          GOTOOLCHAIN = "local";
        };
      });
    };
}
