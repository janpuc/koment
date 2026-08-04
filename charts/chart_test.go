package charts_test

import (
	"os/exec"
	"strings"
	"testing"
)

const chart = "koment"

func render(t *testing.T, values ...string) string {
	t.Helper()
	arguments := append([]string{"template", "koment", chart}, values...)
	command := exec.Command("helm", arguments...)
	rendered, err := command.CombinedOutput()
	if err != nil {
		if _, missing := exec.LookPath("helm"); missing != nil {
			t.Fatalf("helm is required to render the chart; run this through `mise run test`: %v", missing)
		}
		t.Fatalf("helm %s: %v\n%s", strings.Join(arguments, " "), err, rendered)
	}
	return string(rendered)
}

// The chart README promises that neither secret reaches a Pod environment
// variable or a rendered manifest. Mounting them as files is what keeps a token
// out of `kubectl describe` and out of anything that dumps the release.
func TestNoSecretEverReachesAnEnvironmentVariableOrTheRenderedManifest(t *testing.T) {
	rendered := render(t,
		"--set", "github.existingSecret=provider",
		"--set", "auth.existingSecret=agents",
		"--set", "metrics.enabled=true",
	)

	for _, forbidden := range []string{"env:", "envFrom:", "valueFrom:", "secretKeyRef:"} {
		if strings.Contains(rendered, forbidden) {
			t.Errorf("the chart renders %q; secrets must stay files, never environment", forbidden)
		}
	}
	for _, mounted := range []string{"/secrets/github/github-token", "/secrets/auth/credentials.yaml"} {
		if !strings.Contains(rendered, mounted) {
			t.Errorf("the chart no longer mounts %s as a file", mounted)
		}
	}
}

// A token flag that survives when its secret is gone would point the service at
// a path nothing mounts, and it would fail at synchronization rather than at
// configuration.
func TestTheProviderTokenFlagAppearsOnlyWithItsSecret(t *testing.T) {
	if flagged := render(t); strings.Contains(flagged, "--github-token-file") {
		t.Error("the chart passes --github-token-file without github.existingSecret")
	}
	withSecret := render(t, "--set", "github.existingSecret=provider")
	if !strings.Contains(withSecret, `"--github-token-file","/secrets/github/github-token"`) {
		t.Error("github.existingSecret no longer authenticates the synchronizer")
	}
}

// ADR 0105: loopback is the only unauthenticated network boundary, so the
// shipped default must not trust anything else.
func TestTheDefaultInstallTrustsOnlyLoopback(t *testing.T) {
	rendered := render(t)

	if !strings.Contains(rendered, `"--trusted-proxies","127.0.0.1/32"`) {
		t.Error("the default install no longer restricts asserted identity to loopback")
	}
	if strings.Contains(rendered, "--human-writes") {
		t.Error("the default install allows proxy-asserted humans to write")
	}
}

// Metrics carry repository names and annotation counts. Keeping them on their
// own listener is what stops an ingress for the application port from exposing
// them with it.
func TestMetricsStayOnTheirOwnListener(t *testing.T) {
	rendered := render(t, "--set", "metrics.enabled=true")

	if !strings.Contains(rendered, `"--metrics","0.0.0.0:9090"`) {
		t.Fatal("metrics no longer bind their own listener")
	}
	if strings.Contains(rendered, `"--metrics","0.0.0.0:8080"`) {
		t.Error("metrics share the application listener")
	}
	if off := render(t); strings.Contains(off, "--metrics") {
		t.Error("metrics are exposed without metrics.enabled")
	}
}

// A container that can escalate or write its own filesystem is a different
// product from the one this chart claims to ship.
func TestEveryRenderedPodIsHardened(t *testing.T) {
	rendered := render(t, "--set", "metrics.enabled=true")

	for _, hardening := range []string{
		"runAsNonRoot: true",
		"allowPrivilegeEscalation: false",
		"readOnlyRootFilesystem: true",
		"type: RuntimeDefault",
	} {
		if !strings.Contains(rendered, hardening) {
			t.Errorf("the rendered chart no longer sets %q", hardening)
		}
	}
	if strings.Contains(rendered, "privileged: true") {
		t.Error("the chart renders a privileged container")
	}
}

// Kubernetes refuses runAsNonRoot when it cannot prove the image user is not
// root, and the curl image names its user rather than numbering it.
func TestTheTestPodPinsANumericUserSoRunAsNonRootCanBeVerified(t *testing.T) {
	rendered := render(t)

	if !strings.Contains(rendered, "runAsUser: 101") || !strings.Contains(rendered, "runAsGroup: 102") {
		t.Error("the helm test pod no longer pins the numeric ids of its pinned image")
	}
}

// `helm test --logs` reads the pod after the hook finishes. Deleting it on
// success races that read, and the race is invisible until it loses.
func TestTheTestPodSurvivesLongEnoughToReadItsLogs(t *testing.T) {
	rendered := render(t)

	if !strings.Contains(rendered, "helm.sh/hook-delete-policy: before-hook-creation") {
		t.Fatal("the helm test pod no longer declares a delete policy")
	}
	if strings.Contains(rendered, "hook-succeeded") {
		t.Error("hook-succeeded deletes the test pod before `helm test --logs` can read it")
	}
}

// The chart is installed by digest so a moving tag cannot change what runs.
func TestTheTestClientIsPinnedByDigest(t *testing.T) {
	if !strings.Contains(render(t), "curlimages/curl@sha256:") {
		t.Error("the helm test client is no longer digest-pinned")
	}
}
