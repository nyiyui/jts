{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-24.11";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
      ...
    }@attrs:
    flake-utils.lib.eachSystem flake-utils.lib.defaultSystems (
      system:
      let
        pkgs = import nixpkgs { inherit system; };
      in
      rec {
        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            nixfmt-rfc-style
          ];
          nativeBuildInputs = with pkgs; [
            gtk4
            gobject-introspection
            pkg-config
            cambalache
            go
            sqlitebrowser
            gdb
            goose
          ];
        };
      }
    );
}
