{
  inputs = {
    nixpkgs.url     = "github:NixOS/nixpkgs/nixos-25.11";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils, ... }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
        
        # Fetch libdogecoin package definition
        libdogecoin = pkgs.callPackage (pkgs.fetchurl {
          url = "https://raw.githubusercontent.com/Dogebox-WG/dogebox-nur-packages/refs/heads/main/pkgs/libdogecoin/default.nix";
          sha256 = "sha256-Rx6w9RKFFkJKOhk9o/aQUDtfnteTMe2Rme+kvnGQr2k=";
        }) {};
        
        optee_libdogecoin = libdogecoin."libdogecoin-optee-host";
        
        # Function to build dkm with optional OP-TEE support
        buildDkm = { withOptee ? false }: pkgs.buildGoModule {
          name = "dkm${if withOptee then "-optee" else ""}";
          src = ./.;

          vendorHash = "sha256-9smxGxt+XHXc6KZnGxCQ9SlFGPu7BmsLATV/O4fybFU=";

          buildPhase = "make";

          nativeBuildInputs = [ pkgs.go_1_25 ];
          
          buildInputs = if withOptee then [
            optee_libdogecoin
          ] else [];
          
          # Make optee_libdogecoin available in PATH when building with OP-TEE
          preBuild = if withOptee then ''
            export PATH="${optee_libdogecoin}/bin:$PATH"
          '' else "";

          installPhase = ''
            mkdir -p $out/bin
            cp dkm $out/bin/
            ${if withOptee then ''
              # Create a wrapper that ensures optee_libdogecoin is in PATH
              mv $out/bin/dkm $out/bin/.dkm-wrapped
              cat > $out/bin/dkm << EOF
            #!/bin/sh
            export PATH="${optee_libdogecoin}/bin:\$PATH"
            exec $out/bin/.dkm-wrapped "\$@"
            EOF
              chmod +x $out/bin/dkm
            '' else ""}
          '';

          meta = with pkgs.lib; {
            description = "Doge Key Manager${if withOptee then " with OP-TEE secure enclave support" else ""}";
            homepage = "https://github.com/dogeorg/dkm";
            license = licenses.mit;
            maintainers = with maintainers; [ dogecoinfoundation ];
            platforms = platforms.all;
          };
        };
      in {
        devShells.default = pkgs.mkShell {
          buildInputs = [
            pkgs.go_1_25
          ];
        };

        devShells.optee = pkgs.mkShell {
          buildInputs = [
            pkgs.go_1_25
            optee_libdogecoin
          ];
          shellHook = ''
            export PATH="${optee_libdogecoin}/bin:$PATH"
            echo "OP-TEE development environment"
            echo "optee_libdogecoin: ${optee_libdogecoin}/bin/optee_libdogecoin"
          '';
        };

        packages = rec {
          dkm = buildDkm { };
          dkm-optee = buildDkm { withOptee = true; };
          default = dkm;
        };

        dbxSessionName = "dkm";
        dbxStartCommand = "make dev";
      }
    );
}
