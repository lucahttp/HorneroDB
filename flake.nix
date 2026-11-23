{
  description = "HorneroDB development environment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
  };

  outputs = { self, nixpkgs }:
    let
      system = "aarch64-darwin";
      pkgs = nixpkgs.legacyPackages.${system};
    in {
      devShells.${system}.default = pkgs.mkShell {
        buildInputs = [
          pkgs.docker-client
        ];

        shellHook = ''
          echo "Starting HorneroDB environment..."

          # Docker socket
          export DOCKER_HOST="unix://$HOME/.socktainer/container.sock"

          # Start Apple Container API
          container system start > /dev/null 2>&1

          # Wait for API to be ready
          sleep 2

          # Start socktainer in background
          socktainer --no-check-compatibility > $HOME/.socktainer/socktainer.log 2>&1 &
          SOCKTAINER_PID=$!

          # Wait for socket to be ready
          sleep 2

          # Cleanup on exit
          trap "echo 'Stopping socktainer...'; kill $SOCKTAINER_PID 2>/dev/null" EXIT

          echo "Environment ready."
          echo "  DOCKER_HOST: $DOCKER_HOST"
          echo "  Logs: $HOME/.socktainer/socktainer.log"
        '';
      };
    };
}
