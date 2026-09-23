package shape_test

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/logkrama/logkrama/internal/loggen"
	"github.com/logkrama/logkrama/internal/normalize"
	"github.com/logkrama/logkrama/internal/normalize/shape"
	"github.com/logkrama/logkrama/internal/parse"
	"github.com/logkrama/logkrama/internal/parse/ops"
	"github.com/logkrama/logkrama/internal/schema"
)

// TestSameEventFourShapes runs one real PAN-OS event through the actual
// parse+normalize pipeline and renders it as UES, ECS, OCSF and CEF — the
// literal "same event, four shapes" demo moment (PS requirement (g)).
func TestSameEventFourShapes(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	registry := parse.NewRegistry(ops.Deps{})
	if _, err := registry.LoadDir(filepath.Join(root, "packs")); err != nil {
		t.Fatalf("load packs: %v", err)
	}
	mappings, err := normalize.LoadMappingsDir(filepath.Join(root, "packs"))
	if err != nil {
		t.Fatalf("load mappings: %v", err)
	}
	dicts, err := normalize.LoadDictionaries(filepath.Join(root, "config", "dictionaries"))
	if err != nil {
		t.Fatalf("load dictionaries: %v", err)
	}

	gen := loggen.NewGenerator(99)
	now := time.Date(2026, 9, 7, 14, 2, 11, 0, time.UTC)
	line := gen.Line(loggen.VendorPaloAlto, now)

	plan, _ := registry.Get("paloalto.panos.traffic")
	res := plan.Run([]byte(line))
	mapper := normalize.NewMapper(mappings["paloalto.panos.traffic"], dicts)
	event, _ := mapper.Apply(res.Fields, now)
	event.Raw.SHA256 = "9a134e421a5579eba9617f5817d6f2929755b22f2c12eef31b4a89a955589bf5"

	ues := shape.UES(event)
	if ues.Src.IP != event.Src.IP {
		t.Fatal("UES shape should be the event itself")
	}

	ecs := shape.ECS(event)
	ecsSrc, ok := ecs["source"].(map[string]any)
	if !ok || ecsSrc["ip"] != event.Src.IP {
		t.Fatalf("ECS shape missing source.ip: %+v", ecs)
	}
	ecsEvent, ok := ecs["event"].(map[string]any)
	if !ok || ecsEvent["dataset"] != "paloalto.panos.traffic" {
		t.Fatalf("ECS shape missing event.dataset: %+v", ecs)
	}

	ocsf := shape.OCSF(event)
	if ocsf["class_uid"] != 4001 {
		t.Fatalf("OCSF shape wrong class_uid: %+v", ocsf["class_uid"])
	}
	ocsfSrc, ok := ocsf["src_endpoint"].(map[string]any)
	if !ok || ocsfSrc["ip"] != event.Src.IP {
		t.Fatalf("OCSF shape missing src_endpoint.ip: %+v", ocsf)
	}

	cef := shape.CEF(event)
	if !strings.HasPrefix(cef, "CEF:0|paloalto|panos|") {
		t.Fatalf("CEF shape malformed header: %s", cef)
	}
	if !strings.Contains(cef, "src="+event.Src.IP) {
		t.Fatalf("CEF shape missing src extension: %s", cef)
	}
	if !strings.Contains(cef, "dst="+event.Dst.IP) {
		t.Fatalf("CEF shape missing dst extension: %s", cef)
	}

	t.Logf("UES  src.ip=%s dst.ip=%s action=%s", ues.Src.IP, ues.Dst.IP, ues.Event.Action)
	t.Logf("ECS  %+v", ecs)
	t.Logf("OCSF %+v", ocsf)
	t.Logf("CEF  %s", cef)
}

func TestCEFEscapesSpecialCharacters(t *testing.T) {
	e := &schema.Event{
		Observer: schema.Observer{Vendor: "vendor|with|pipes", Product: `back\slash`},
		Event:    schema.EventMeta{Action: "allow=deny"},
	}
	cef := shape.CEF(e)
	if !strings.Contains(cef, `vendor\|with\|pipes`) {
		t.Errorf("pipe in header field not escaped: %s", cef)
	}
	if !strings.Contains(cef, `back\\slash`) {
		t.Errorf("backslash in header field not escaped: %s", cef)
	}
	if !strings.Contains(cef, `act=allow\=deny`) {
		t.Errorf("equals in extension value not escaped: %s", cef)
	}
}
