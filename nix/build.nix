# Build functions for Crossplane.
#
# All functions are builders that take an attrset of arguments.
# This makes dependencies explicit and keeps flake.nix as a clean manifest.
#
# Key primitives used here:
#   pkgs.buildGoApplication - gomod2nix's Go builder (https://github.com/nix-community/gomod2nix)
#   pkgs.dockerTools        - Build OCI images without Docker (https://nixos.org/manual/nixpkgs/stable/#sec-pkgs-dockerTools)
#   pkgs.runCommand         - Run a shell script, capture output directory as $out
{ pkgs, self }:
let
  # Build a Go binary for a specific platform.
  goBinary =
    { version, pname, subPackage, platform }:
    let
      ext = if platform.os == "windows" then ".exe" else "";
    in
    pkgs.buildGoApplication {
      pname = "${pname}-${platform.os}-${platform.arch}";
      inherit version;
      src = self;
      pwd = self;
      modules = "${self}/gomod2nix.toml";
      subPackages = [ subPackage ];

      # Cross-compile by merging GOOS/GOARCH into Go's attrset (// merges attrsets).
      go = pkgs.go // {
        GOOS = platform.os;
        GOARCH = platform.arch;
      };

      CGO_ENABLED = "0";
      doCheck = false;

      preBuild = ''
        ldflags="-s -w -X=github.com/crossplane/crossplane/v2/internal/version.version=${version}"
      '';

      postInstall = ''
        if [ -d $out/bin/${platform.os}_${platform.arch} ]; then
          mv $out/bin/${platform.os}_${platform.arch}/* $out/bin/
          rmdir $out/bin/${platform.os}_${platform.arch}
        fi
        cd $out/bin
        sha256sum ${pname}${ext} | head -c 64 > ${pname}${ext}.sha256
      '';

      meta = {
        description = "Crossplane - The cloud native control plane framework";
        homepage = "https://crossplane.io";
        license = pkgs.lib.licenses.asl20;
        mainProgram = pname;
      };
    };

  # Build OCI image arguments for dockerTools.
  mkImageArgs =
    { version, crossplaneBin, arch }:
    let
      passwd = pkgs.writeText "passwd" ''
        root:x:0:0:root:/root:/sbin/nologin
        nobody:x:65534:65534:nobody:/nonexistent:/sbin/nologin
        nonroot:x:65532:65532:nonroot:/home/nonroot:/sbin/nologin
      '';
      group = pkgs.writeText "group" ''
        root:x:0:
        nobody:x:65534:
        nonroot:x:65532:
      '';
      nsswitch = pkgs.writeText "nsswitch.conf" ''
        hosts: files dns
      '';
    in
    {
      name = "crossplane/crossplane";
      tag = version;
      created = "now";
      architecture = arch;

      contents = [
        crossplaneBin
        pkgs.cacert
        pkgs.tzdata
        pkgs.iana-etc
      ];

      extraCommands = ''
        mkdir -p tmp home/nonroot etc crds webhookconfigurations
        chmod 1777 tmp
        cp ${passwd} etc/passwd
        cp ${group} etc/group
        cp ${nsswitch} etc/nsswitch.conf
        cp -r ${self}/cluster/crds/* crds/
        cp -r ${self}/cluster/webhookconfigurations/* webhookconfigurations/
      '';

      config = {
        Entrypoint = [ "/bin/crossplane" ];
        ExposedPorts = { "8080/tcp" = { }; };
        User = "65532";
        Env = [
          "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
          "SSL_CERT_FILE=${pkgs.cacert}/etc/ssl/certs/ca-certificates.crt"
        ];
        Labels = {
          "org.opencontainers.image.source" = "https://github.com/crossplane/crossplane";
          "org.opencontainers.image.version" = version;
        };
      };
    };

  # Build crank tarball with checksums.
  crankBundle =
    { version, crankDrv, platform }:
    let
      ext = if platform.os == "windows" then ".exe" else "";
    in
    pkgs.runCommand "crank-bundle-${platform.os}-${platform.arch}-${version}"
      { nativeBuildInputs = [ pkgs.gnutar pkgs.gzip ]; }
      ''
        mkdir -p $out
        cp ${crankDrv}/bin/crank${ext} .
        cp ${crankDrv}/bin/crank${ext}.sha256 .
        tar -czvf $out/crank.tar.gz crank${ext} crank${ext}.sha256
        cd $out
        sha256sum crank.tar.gz | head -c 64 > crank.tar.gz.sha256
      '';

in
{
  # OCI images for all Linux platforms.
  images =
    { version, platforms }:
    builtins.listToAttrs (
      map (p: {
        name = "${p.os}-${p.arch}";
        value = {
          bin = goBinary {
            inherit version;
            pname = "crossplane";
            subPackage = "cmd/crossplane";
            platform = p;
          };
          image = pkgs.dockerTools.buildLayeredImage (
            mkImageArgs {
              inherit version;
              arch = p.arch;
              crossplaneBin = goBinary {
                inherit version;
                pname = "crossplane";
                subPackage = "cmd/crossplane";
                platform = p;
              };
            }
          );
        };
      }) platforms
    );

  # Helm chart package.
  chart =
    { version }:
    let
      chartVersion = builtins.substring 1 (-1) version;
    in
    pkgs.runCommand "crossplane-helm-chart-${chartVersion}"
      { nativeBuildInputs = [ pkgs.kubernetes-helm ]; }
      ''
        mkdir -p $out
        cp -r ${self}/cluster/charts/crossplane chart
        chmod -R u+w chart
        cd chart
        helm dependency update 2>/dev/null || true
        helm package --version ${chartVersion} --app-version ${chartVersion} -d $out .
      '';

  # E2E test binary.
  e2e =
    { version }:
    pkgs.buildGoApplication {
      pname = "crossplane-e2e";
      inherit version;
      src = self;
      pwd = self;
      modules = "${self}/gomod2nix.toml";

      CGO_ENABLED = "0";

      buildPhase = ''
        runHook preBuild
        go test -c -o e2e ./test/e2e
        runHook postBuild
      '';

      installPhase = ''
        mkdir -p $out/bin
        cp e2e $out/bin/
      '';

      doCheck = false;
    };

  # Image args for streaming (used by apps.streamImage).
  imageArgs =
    { version, arch }:
    mkImageArgs {
      inherit version arch;
      crossplaneBin = goBinary {
        inherit version;
        pname = "crossplane";
        subPackage = "cmd/crossplane";
        platform = { os = "linux"; inherit arch; };
      };
    };

  # Full release package with all artifacts.
  release =
    { version, goPlatforms, imagePlatforms }:
    let
      chartVersion = builtins.substring 1 (-1) version;

      crossplaneBins = builtins.listToAttrs (
        map (p: {
          name = "${p.os}-${p.arch}";
          value = goBinary {
            inherit version;
            pname = "crossplane";
            subPackage = "cmd/crossplane";
            platform = p;
          };
        }) goPlatforms
      );

      crankBins = builtins.listToAttrs (
        map (p: {
          name = "${p.os}-${p.arch}";
          value = goBinary {
            inherit version;
            pname = "crank";
            subPackage = "cmd/crank";
            platform = p;
          };
        }) goPlatforms
      );

      crossplaneImages = builtins.listToAttrs (
        map (p: {
          name = "${p.os}-${p.arch}";
          value = {
            bin = crossplaneBins."${p.os}-${p.arch}";
            image = pkgs.dockerTools.buildLayeredImage (
              mkImageArgs {
                inherit version;
                arch = p.arch;
                crossplaneBin = crossplaneBins."${p.os}-${p.arch}";
              }
            );
          };
        }) imagePlatforms
      );

      crankBundles = builtins.listToAttrs (
        map (p: {
          name = "${p.os}-${p.arch}";
          value = crankBundle {
            inherit version;
            crankDrv = crankBins."${p.os}-${p.arch}";
            platform = p;
          };
        }) goPlatforms
      );

      chart = pkgs.runCommand "crossplane-helm-chart-${chartVersion}"
        { nativeBuildInputs = [ pkgs.kubernetes-helm ]; }
        ''
          mkdir -p $out
          cp -r ${self}/cluster/charts/crossplane chart
          chmod -R u+w chart
          cd chart
          helm dependency update 2>/dev/null || true
          helm package --version ${chartVersion} --app-version ${chartVersion} -d $out .
        '';
    in
    pkgs.runCommand "crossplane-release-${version}" { } ''
      mkdir -p $out/bin $out/bundle $out/charts $out/images

      ${pkgs.lib.concatMapStrings (p: ''
        mkdir -p $out/bin/${p.os}_${p.arch}
        cp ${crossplaneBins."${p.os}-${p.arch}"}/bin/* $out/bin/${p.os}_${p.arch}/
        cp ${crankBins."${p.os}-${p.arch}"}/bin/* $out/bin/${p.os}_${p.arch}/
      '') goPlatforms}

      ${pkgs.lib.concatMapStrings (p: ''
        mkdir -p $out/bundle/${p.os}_${p.arch}
        cp ${crankBundles."${p.os}-${p.arch}"}/* $out/bundle/${p.os}_${p.arch}/
      '') goPlatforms}

      cp ${chart}/* $out/charts/

      ${pkgs.lib.concatMapStrings (p: ''
        mkdir -p $out/images/${p.os}_${p.arch}
        cp ${crossplaneImages."${p.os}-${p.arch}".image} $out/images/${p.os}_${p.arch}/image.tar.gz
      '') imagePlatforms}
    '';
}
