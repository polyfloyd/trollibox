{
  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs?ref=nixos-unstable";
  };

  outputs =
    { self, nixpkgs }:
    {
      devShells = builtins.mapAttrs (system: pkgs: {
        default = pkgs.mkShell {
          packages = with pkgs; [
            commitizen
            go
            golangci-lint
            mpc
            mpd
            nodejs
          ];
        };
      }) nixpkgs.legacyPackages;
    };
}
