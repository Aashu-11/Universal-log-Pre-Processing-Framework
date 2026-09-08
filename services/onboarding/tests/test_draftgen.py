from pathlib import Path

import yaml

from app.draftgen import generate_draft

REPO_ROOT = str(Path(__file__).resolve().parents[3])


def test_selects_json_shape():
    lines = ['{"src": "1.2.3.4", "dst": "5.6.7.8", "action": "allow"}'] * 10
    draft = generate_draft(
        lines, "test.json.demo", "test", "demo", "traffic", REPO_ROOT, ""
    )
    assert draft.shape == "json"
    doc = yaml.safe_load(draft.parser_yaml)
    assert doc["pipeline"][0]["op"] == "json"


def test_selects_kv_shape():
    lines = ['srcip=1.2.3.4 dstip=5.6.7.8 action="allow" bytes=100'] * 10
    draft = generate_draft(
        lines, "test.kv.demo", "test", "demo", "traffic", REPO_ROOT, ""
    )
    assert draft.shape == "kv"
    doc = yaml.safe_load(draft.parser_yaml)
    assert doc["pipeline"][0]["op"] == "kv"


def test_selects_csv_shape():
    lines = ["1.2.3.4,5.6.7.8,443,allow"] * 10
    draft = generate_draft(
        lines, "test.csv.demo", "test", "demo", "traffic", REPO_ROOT, ""
    )
    assert draft.shape == "csv"
    doc = yaml.safe_load(draft.parser_yaml)
    assert doc["pipeline"][0]["op"] == "csv"


def test_selects_dissect_fallback_for_freeform_text():
    lines = [f"connection from 1.2.3.{i} to 5.6.7.8 on port 443" for i in range(10)]
    draft = generate_draft(
        lines, "test.dissect.demo", "test", "demo", "traffic", REPO_ROOT, "connection"
    )
    assert draft.shape == "dissect"
    doc = yaml.safe_load(draft.parser_yaml)
    assert doc["pipeline"][0]["op"] == "dissect"
    assert "%{" in doc["pipeline"][0]["pattern"]


def test_mapping_fields_include_dictionary_for_verdict():
    lines = ['srcip=1.2.3.4 dstip=5.6.7.8 action="allow"'] * 10
    draft = generate_draft(
        lines, "test.kv.verdict", "test", "demo", "traffic", REPO_ROOT, ""
    )
    mapping_doc = yaml.safe_load(draft.mapping_yaml)
    action_field = mapping_doc["fields"].get("event.action")
    assert action_field is not None
    assert action_field["dictionary"] == "action"


def test_generated_yaml_is_valid_yaml_for_all_shapes():
    for lines, expected_shape in [
        (['{"a": "1"}'] * 5, "json"),
        (["a=1 b=2"] * 5, "kv"),
        (["1,2,3"] * 5, "csv"),
    ]:
        draft = generate_draft(
            lines, f"test.{expected_shape}.valid", "t", "p", "c", REPO_ROOT, ""
        )
        parser_doc = yaml.safe_load(draft.parser_yaml)
        mapping_doc = yaml.safe_load(draft.mapping_yaml)
        assert parser_doc["metadata"]["id"] == f"test.{expected_shape}.valid"
        assert mapping_doc["metadata"]["parser_id"] == f"test.{expected_shape}.valid"
