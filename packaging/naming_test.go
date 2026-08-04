package packaging_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

const versionPlaceholder = "VERSION"

var (
	buildMatrix     = regexp.MustCompile(`for target in ([^;]+); do`)
	archiveName     = regexp.MustCompile(`name="koment_` + versionPlaceholder + `_\$\{os\}_\$\{arch\}"`)
	archiveMentions = regexp.MustCompile(`koment_` + versionPlaceholder + `_([a-z0-9]+)_([a-z0-9]+)\.(tar\.gz|zip)`)
	checksumMention = regexp.MustCompile(`koment_` + versionPlaceholder + `_checksums\.txt`)
)

var versionTokens = strings.NewReplacer(
	"${VERSION}", versionPlaceholder,
	"{{VERSION}}", versionPlaceholder,
	"${version}", versionPlaceholder,
	"$version", versionPlaceholder,
)

func repositoryFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("..", filepath.FromSlash(path)))
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return versionTokens.Replace(string(content))
}

type archive struct {
	os, arch, extension string
}

func (a archive) String() string {
	return fmt.Sprintf("koment_%s_%s_%s.%s", versionPlaceholder, a.os, a.arch, a.extension)
}

func (a archive) binary() string {
	if a.os == "windows" {
		return "koment.exe"
	}
	return "koment"
}

const releaseWorkflow = ".github/workflows/release.yml"

func published(t *testing.T) map[archive]bool {
	t.Helper()
	workflow := repositoryFile(t, releaseWorkflow)

	if !archiveName.MatchString(workflow) {
		t.Fatalf("%s no longer builds koment_%s_<os>_<arch>; every consumer below is now guessing",
			releaseWorkflow, versionPlaceholder)
	}
	matrix := buildMatrix.FindStringSubmatch(workflow)
	if matrix == nil {
		t.Fatalf("%s has no recognisable build matrix", releaseWorkflow)
	}

	archives := map[archive]bool{}
	for _, target := range strings.Fields(matrix[1]) {
		platform, architecture, found := strings.Cut(target, "/")
		if !found {
			t.Fatalf("build target %q is not os/arch", target)
		}
		archives[archive{os: platform, arch: architecture, extension: extensionFor(platform)}] = true
	}
	if len(archives) == 0 {
		t.Fatal("the release workflow builds nothing")
	}
	return archives
}

func extensionFor(platform string) string {
	if platform == "windows" {
		return "zip"
	}
	return "tar.gz"
}

var consumers = map[string]string{
	"the Homebrew tap":  "packaging/homebrew/koment.rb.tmpl",
	"the Scoop bucket":  "packaging/scoop/koment.json.tmpl",
	"the WinGet bundle": "packaging/winget/JanPuc.Koment.installer.yaml.tmpl",
}

const setupAction = "action.yml"

var (
	actionArchive     = regexp.MustCompile(`archive="koment_` + versionPlaceholder + `_\$\{platform\}_\$\{arch\}\.(tar\.gz|zip)"`)
	actionPlatform    = regexp.MustCompile(`platform=([a-z0-9]+)\s*;;`)
	actionArchitecure = regexp.MustCompile(`arch=([a-z0-9]+)\s*;;`)
)

func requestedByTheSetupAction(t *testing.T) map[archive]bool {
	t.Helper()
	content := repositoryFile(t, setupAction)

	template := actionArchive.FindStringSubmatch(content)
	if template == nil {
		t.Fatalf("%s no longer builds its download name from the runner's platform and architecture", setupAction)
	}
	platforms := actionPlatform.FindAllStringSubmatch(content, -1)
	architectures := actionArchitecure.FindAllStringSubmatch(content, -1)
	if platforms == nil || architectures == nil {
		t.Fatalf("%s has no recognisable runner mapping", setupAction)
	}

	requested := map[archive]bool{}
	for _, platform := range platforms {
		for _, architecture := range architectures {
			requested[archive{os: platform[1], arch: architecture[1], extension: template[1]}] = true
		}
	}
	return requested
}

func TestEveryPackagedArchiveIsOneTheReleaseWorkflowBuilds(t *testing.T) {
	archives := published(t)

	for consumer, path := range consumers {
		content := repositoryFile(t, path)
		mentions := archiveMentions.FindAllStringSubmatch(content, -1)
		if mentions == nil {
			t.Errorf("%s (%s) references no koment archive at all", consumer, path)
			continue
		}
		for _, mention := range mentions {
			referenced := archive{os: mention[1], arch: mention[2], extension: mention[3]}
			if !archives[referenced] {
				t.Errorf("%s (%s) downloads %s, which the release workflow never builds; buildable: %s",
					consumer, path, referenced, sortedNames(archives))
			}
		}
	}

	for requested := range requestedByTheSetupAction(t) {
		if !archives[requested] {
			t.Errorf("the setup action (%s) resolves runners to %s, which the release workflow never builds; buildable: %s",
				setupAction, requested, sortedNames(archives))
		}
	}
}

// An archive nothing installs is either a wasted build or a platform whose
// install path was forgotten. Both are worth failing over.
func TestEveryPublishedArchiveHasSomethingThatInstallsIt(t *testing.T) {
	archives := published(t)

	installed := map[archive]string{}
	for consumer, path := range consumers {
		content := repositoryFile(t, path)
		for _, mention := range archiveMentions.FindAllStringSubmatch(content, -1) {
			installed[archive{os: mention[1], arch: mention[2], extension: mention[3]}] = consumer
		}
	}
	for requested := range requestedByTheSetupAction(t) {
		installed[requested] = "the setup action"
	}

	for built := range archives {
		if installed[built] == "" {
			t.Errorf("the release workflow builds %s but no packaging channel installs it", built)
		}
	}
}

// The setup action verifies its download against the manifest the release
// workflow writes, so the two must agree on that file's name too.
func TestTheSetupActionAsksForTheChecksumManifestByItsPublishedName(t *testing.T) {
	if !checksumMention.MatchString(repositoryFile(t, releaseWorkflow)) {
		t.Fatalf("%s no longer writes koment_%s_checksums.txt", releaseWorkflow, versionPlaceholder)
	}
	if !checksumMention.MatchString(repositoryFile(t, "action.yml")) {
		t.Error("action.yml would download a checksum manifest the release never publishes")
	}
}

// Package managers put the extracted binary on PATH by name, so the name inside
// the archive is as much a public interface as the archive itself.
func TestPackagedBinaryNamesMatchWhatTheArchivesContain(t *testing.T) {
	for _, expectation := range []struct {
		consumer, path, needle string
		platform               string
	}{
		{"the Homebrew tap", "packaging/homebrew/koment.rb.tmpl", `bin.install "koment"`, "darwin"},
		{"the WinGet bundle", "packaging/winget/JanPuc.Koment.installer.yaml.tmpl", "RelativeFilePath: koment.exe", "windows"},
		{"the Scoop bucket", "packaging/scoop/koment.json.tmpl", `"bin": "koment.exe"`, "windows"},
		{"the setup action", "action.yml", `-C "$target" koment`, "linux"},
	} {
		want := archive{os: expectation.platform}.binary()
		if !strings.Contains(expectation.needle, want) {
			t.Fatalf("test is inconsistent: %q does not name %q", expectation.needle, want)
		}
		if !strings.Contains(repositoryFile(t, expectation.path), expectation.needle) {
			t.Errorf("%s (%s) no longer installs %q; the release workflow packages that name for %s",
				expectation.consumer, expectation.path, want, expectation.platform)
		}
	}
}

// Homebrew installs from the archive root and the setup action extracts a bare
// member name, so nesting the binary in a directory would break both without
// changing a single file name (ADR 0109).
func TestBothArchiveShapesCarryTheBinaryAndItsLicenceAtTheRoot(t *testing.T) {
	workflow := repositoryFile(t, releaseWorkflow)

	for shape, packaging := range map[string]string{
		"the Windows zip":     `zip -q "${name}.zip" "$binary" LICENSE README.md`,
		"the POSIX tarball":   `tar -czf "${name}.tar.gz" "$binary" LICENSE README.md`,
		"the Windows binary":  `[ "$os" = windows ] && binary=koment.exe`,
		"the extracted entry": `tar -xzf "${work}/${archive}" -C "$target" koment`,
	} {
		source := releaseWorkflow
		if shape == "the extracted entry" {
			source = setupAction
		}
		if !strings.Contains(repositoryFile(t, source), packaging) {
			t.Errorf("%s no longer packages the binary, LICENSE and README at the archive root: %s expected %q",
				shape, source, packaging)
		}
	}

	if strings.Contains(workflow, `tar -czf "${name}.tar.gz" -C`) {
		t.Error("the tarball now nests its contents; Homebrew's bin.install and the setup action both read the archive root")
	}
}

func sortedNames(archives map[archive]bool) string {
	names := make([]string, 0, len(archives))
	for a := range archives {
		names = append(names, a.String())
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
