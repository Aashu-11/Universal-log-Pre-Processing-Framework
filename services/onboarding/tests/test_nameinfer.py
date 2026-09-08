from app.nameinfer import (
    infer_field_names,
    looks_like_csv,
    looks_like_json,
    looks_like_kv,
)


def test_infers_names_from_preceding_literal_tokens():
    names = infer_field_names("connection from <*> to <*> on port <*>")
    assert names == ["from", "to", "port"]


def test_dedupes_repeated_names():
    names = infer_field_names("<*> connected to <*> then to <*>")
    assert names[1] == "to"
    assert names[2] == "to_2"


def test_falls_back_to_field_n_when_no_literal_precedes():
    names = infer_field_names("<*> started the process")
    assert names == ["field_1"]


def test_looks_like_kv_true():
    lines = ['srcip=1.2.3.4 dstip=5.6.7.8 action="allow"'] * 5
    assert looks_like_kv(lines) is True


def test_looks_like_kv_false_for_prose():
    lines = ["this is just a sentence with no equals signs at all"] * 5
    assert looks_like_kv(lines) is False


def test_looks_like_csv_true():
    lines = ["a,b,c,d", "1,2,3,4", "x,y,z,w"]
    assert looks_like_csv(lines) is True


def test_looks_like_csv_false_when_column_count_varies():
    lines = ["a,b,c", "1,2,3,4,5"]
    assert looks_like_csv(lines) is False


def test_looks_like_json_true():
    lines = ['{"a": 1}', '{"b": 2, "c": 3}']
    assert looks_like_json(lines) is True


def test_looks_like_json_false():
    lines = ["not json at all"]
    assert looks_like_json(lines) is False
