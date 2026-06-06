package model

import (
	"encoding/base64"
	"strings"
	"testing"
)

func b64(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }

func TestParseK8sSecret_DecodesData(t *testing.T) {
	manifest := `{
	  "apiVersion": "v1",
	  "kind": "Secret",
	  "metadata": {"name": "db-creds"},
	  "type": "Opaque",
	  "data": {"username": "` + b64("admin") + `", "password": "` + b64("s3cr3t") + `"}
	}`
	sec, issues, err := ParseK8sSecret([]byte(manifest))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(issues) != 0 {
		t.Fatalf("unexpected issues: %v", issues)
	}
	if sec.Name != "db-creds" || sec.Type != "Opaque" {
		t.Errorf("metadata: got name=%q type=%q", sec.Name, sec.Type)
	}
	if sec.Data["username"] != "admin" || sec.Data["password"] != "s3cr3t" {
		t.Errorf("decoded data mismatch: %v", sec.Data)
	}
}

func TestParseK8sSecret_InvalidBase64Reported(t *testing.T) {
	manifest := `{"kind":"Secret","metadata":{"name":"x"},"data":{"bad":"!!!not base64!!!"}}`
	sec, issues, err := ParseK8sSecret([]byte(manifest))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(issues) == 0 {
		t.Error("expected an issue for invalid base64 value")
	}
	if _, ok := sec.Data["bad"]; ok {
		t.Error("invalid value should not be present in decoded data")
	}
}

func TestParseK8sSecret_WrongKind(t *testing.T) {
	if _, _, err := ParseK8sSecret([]byte(`{"kind":"ConfigMap","data":{}}`)); err == nil {
		t.Error("expected error for non-Secret kind")
	}
}

func TestInspectK8sSecret_DoesNotEchoValues(t *testing.T) {
	manifest := `{"kind":"Secret","metadata":{"name":"x"},"data":{"token":"` + b64("leak-me") + `"}}`
	insp := inspectK8sSecret("secret.json", []byte(manifest))
	for _, f := range insp.Fields {
		if strings.Contains(f.Value, "leak-me") {
			t.Fatalf("secret value leaked in field %q", f.Key)
		}
	}
}

func TestDetect_K8sSecretJSON(t *testing.T) {
	manifest := `{"apiVersion":"v1","kind":"Secret","metadata":{"name":"x"},"data":{}}`
	if k := Detect("secret.json", []byte(manifest)); k != KindK8sSecret {
		t.Errorf("expected KindK8sSecret, got %v", k)
	}
}

// yamlManifest builds a realistic YAML Secret manifest with synthetic values.
func yamlManifest(name string) string {
	return "apiVersion: v1\n" +
		"kind: Secret\n" +
		"metadata:\n" +
		"  name: " + name + "\n" +
		"type: Opaque\n" +
		"data:\n" +
		"  username: " + b64("admin") + "\n" +
		"  password: " + b64("s3cr3t") + "\n" +
		"stringData:\n" +
		"  config.yaml: |\n" +
		"    host: db.internal\n"
}

func TestParseK8sSecret_YAMLDecodesData(t *testing.T) {
	sec, issues, err := ParseK8sSecret([]byte(yamlManifest("db-creds")))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(issues) != 0 {
		t.Fatalf("unexpected issues: %v", issues)
	}
	if sec.Name != "db-creds" || sec.Type != "Opaque" {
		t.Errorf("metadata: got name=%q type=%q", sec.Name, sec.Type)
	}
	if sec.Data["username"] != "admin" || sec.Data["password"] != "s3cr3t" {
		t.Errorf("decoded data mismatch: %v", sec.Data)
	}
	if _, ok := sec.StringData["config.yaml"]; !ok {
		t.Errorf("expected stringData key config.yaml, got %v", sec.StringData)
	}
}

func TestParseK8sSecret_YAMLWrongKind(t *testing.T) {
	manifest := "apiVersion: v1\nkind: ConfigMap\ndata: {}\n"
	if _, _, err := ParseK8sSecret([]byte(manifest)); err == nil {
		t.Error("expected error for non-Secret kind in YAML")
	}
}

func TestDetect_K8sSecretYAML(t *testing.T) {
	if k := Detect("secret.yaml", []byte(yamlManifest("x"))); k != KindK8sSecret {
		t.Errorf("expected KindK8sSecret for YAML, got %v", k)
	}
	if k := Detect("secret.yml", []byte(yamlManifest("x"))); k != KindK8sSecret {
		t.Errorf("expected KindK8sSecret for .yml, got %v", k)
	}
}

func TestInspectK8sSecret_YAMLSurfacesKeysAndMasksValues(t *testing.T) {
	insp := inspectK8sSecret("secret.yaml", []byte(yamlManifest("x")))
	if !insp.Valid {
		t.Fatalf("expected valid inspection, issues: %v", insp.Issues)
	}
	var sawUsername, sawPassword, sawStringData bool
	for _, f := range insp.Fields {
		// Keys must be surfaced.
		switch f.Key {
		case "data.username":
			sawUsername = true
		case "data.password":
			sawPassword = true
		case "stringData.config.yaml":
			sawStringData = true
		}
		// Values must never be echoed.
		for _, leak := range []string{"admin", "s3cr3t", "db.internal"} {
			if strings.Contains(f.Value, leak) {
				t.Fatalf("secret value %q leaked in field %q", leak, f.Key)
			}
		}
	}
	if !sawUsername || !sawPassword || !sawStringData {
		t.Errorf("missing expected keys: username=%v password=%v stringData=%v",
			sawUsername, sawPassword, sawStringData)
	}
}

func TestDetect_PlainYAMLNotSecret(t *testing.T) {
	// A non-Secret YAML config must not be misclassified as a k8s secret.
	cfg := "host: localhost\nport: 8080\n"
	if k := Detect("config.yaml", []byte(cfg)); k == KindK8sSecret {
		t.Error("plain YAML config should not be detected as a k8s secret")
	}
}
