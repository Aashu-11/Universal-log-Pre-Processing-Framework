import yaml

from app.importers import import_cef_mapping, import_logstash_grok


def test_import_grok_converts_supported_patterns():
    grok = "%{IPORHOST:client} %{WORD:verb} %{NUMBER:duration}"
    parser_yaml, mapping_yaml, unsupported = import_logstash_grok(
        grok, "test.imported.access", "test", "webserver"
    )
    assert unsupported == []
    doc = yaml.safe_load(parser_yaml)
    assert doc["metadata"]["id"] == "test.imported.access"
    pattern = doc["pipeline"][0]["pattern"]
    assert "%{IPORHOST:client}" in pattern
    assert "%{WORD:verb}" in pattern


def test_import_grok_flags_unsupported_patterns_but_still_produces_output():
    grok = "%{COMBINEDAPACHELOG}"
    parser_yaml, mapping_yaml, unsupported = import_logstash_grok(
        grok, "test.imported.apache", "test", "apache"
    )
    assert "COMBINEDAPACHELOG" in unsupported
    doc = yaml.safe_load(parser_yaml)
    assert "GREEDYDATA" in doc["pipeline"][0]["pattern"]


def test_import_cef_mapping_generates_field_mappings():
    field_map = {"src": "src.ip", "dpt": "dst.port"}
    parser_yaml, mapping_yaml = import_cef_mapping(
        field_map, "test.imported.cef", "test", "cefsource"
    )
    parser_doc = yaml.safe_load(parser_yaml)
    assert parser_doc["pipeline"][0]["op"] == "cef"

    mapping_doc = yaml.safe_load(mapping_yaml)
    assert mapping_doc["fields"]["src.ip"]["from"] == "src"
    assert mapping_doc["fields"]["dst.port"]["from"] == "dpt"
