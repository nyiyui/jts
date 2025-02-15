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
        build =
          pkgs:
          package:
          (pkgs.buildGoModule rec {
            pname = "jts-server";
            version = if (self ? rev) then self.rev else "dirty";
            src = ./.;
            vendorHash = "sha256-ZS5KYdFQgeIW8FdT0GXNcQAYVEdhkSD7CGmVcQI36c4=";
            subPackages = [ package ];
            ldflags = [ "-X nyiyui.ca/jts/server.vcsInfo=${version}" ];
          });
      in
      rec {
        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            nixfmt-rfc-style
            go
            sqlitebrowser
            gdb
            goose
            sqlite
          ];
          nativeBuildInputs = with pkgs; [
            gtk4
            libadwaita
            gobject-introspection
            pkg-config
          ];
        };
        packages.default = build pkgs "cmd/server";
        packages.gtkui = build pkgs "cmd/gtkui";
      }
    );
}
