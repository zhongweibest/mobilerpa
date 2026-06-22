package discovery

import (
	"reflect"
	"testing"
)

func TestCollectDisconnectTargetsReturnsEndpointAndResolvedIP(t *testing.T) {
	t.Parallel()

	mdnsOutput := `List of discovered mdns services
adb-device-demo._adb-tls-connect._tcp _adb-tls-connect._tcp 192.168.0.20:42837
`

	targets := collectDisconnectTargets("adb-device-demo._adb-tls-connect._tcp", mdnsOutput)
	expected := []string{
		"adb-device-demo._adb-tls-connect._tcp",
		"192.168.0.20:42837",
	}
	if !reflect.DeepEqual(targets, expected) {
		t.Fatalf("unexpected disconnect targets: %#v", targets)
	}
}

func TestCollectDisconnectTargetsKeepsNetworkEndpointOnly(t *testing.T) {
	t.Parallel()

	targets := collectDisconnectTargets("192.168.0.20:42837", "")
	expected := []string{"192.168.0.20:42837"}
	if !reflect.DeepEqual(targets, expected) {
		t.Fatalf("unexpected disconnect targets: %#v", targets)
	}
}

func TestIsNetworkEndpoint(t *testing.T) {
	t.Parallel()

	if !isNetworkEndpoint("192.168.0.20:42837") {
		t.Fatalf("expected ip:port to be recognized as network endpoint")
	}
	if isNetworkEndpoint("adb-device-demo._adb-tls-connect._tcp") {
		t.Fatalf("expected adb mdns service name to be rejected as network endpoint")
	}
}
