{
  description = "k - a kubectl wrapper that makes common operations easier";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        version = "devel";
      in
      {
        packages.default = pkgs.buildGoModule {
          pname = "k";
          inherit version;
          src = ./.;

          # After first build, replace this with the actual hash nix reports
          vendorHash = null;

          ldflags = [
            "-s"
            "-w"
            "-X main.version=${version}"
          ];

          nativeBuildInputs = [ pkgs.installShellFiles ];

          postInstall = ''
            installShellCompletion --bash completions/bash/k
            installShellCompletion --fish completions/fish/k.fish
            installShellCompletion --zsh completions/zsh/_k
          '';

          meta = with pkgs.lib; {
            description = "A kubectl wrapper that makes common operations easier";
            homepage = "https://github.com/rothgar/k";
            license = licenses.asl20;
            mainProgram = "k";
          };
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [ go gopls kubectl ];
        };
      }
    );
}
