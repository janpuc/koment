# Package-manager metadata

The release workflow renders these templates from the same archives and SHA-256
values attached to the GitHub release. It uploads a Homebrew formula, a Scoop
manifest, and a WinGet manifest bundle beside the canonical artifacts.

The rendered files are submission-ready, but an external catalog is not called
available until its repository or marketplace accepts them. The first release
therefore needs these owner-side steps:

- place `koment.rb` at `Formula/koment.rb` in `janpuc/homebrew-tap`;
- place `koment-scoop.json` at `bucket/koment.json` in a Scoop bucket;
- submit the WinGet bundle to `microsoft/winget-pkgs` with `wingetcreate`.

mise needs no separate manifest and installs the same archives directly through
its GitHub backend with `mise use -g github:janpuc/koment`.
