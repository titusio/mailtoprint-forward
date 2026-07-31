{
  description = "mailtoprint-forward: read IMAP mail and forward attachments to a mail-to-print address";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
      in
      {
        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            go
            gopls
            gotools
            go-tools
          ];

          shellHook = ''
            echo "mailtoprint-forward dev shell — $(go version)"
          '';
        };
      });
}
