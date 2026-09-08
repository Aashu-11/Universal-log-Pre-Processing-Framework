from pathlib import Path

from app.typeinfer import infer_type

REPO_ROOT = str(Path(__file__).resolve().parents[3])


def test_infers_ipv4():
    result = infer_type(["10.0.0.1", "192.168.1.5", "203.0.113.9"], "srcip", REPO_ROOT)
    assert result.type_name == "ipv4"
    assert result.confidence == 1.0


def test_infers_ipv6():
    result = infer_type(["::1", "fe80::1", "2001:db8::1"], "addr", REPO_ROOT)
    assert result.type_name == "ipv6"


def test_infers_mac():
    result = infer_type(["00:11:22:33:44:55", "aa:bb:cc:dd:ee:ff"], "mac", REPO_ROOT)
    assert result.type_name == "mac"


def test_infers_port_only_with_name_hint():
    result = infer_type(["443", "8080", "22"], "dstport", REPO_ROOT)
    assert result.type_name == "port"
    # Without a "port" hint in the field name, the same values should NOT
    # be classified as port (an integer with no semantic hint is just an
    # integer) — this is what keeps port inference from over-firing on any
    # small number.
    result_no_hint = infer_type(["443", "8080", "22"], "count", REPO_ROOT)
    assert result_no_hint.type_name != "port"


def test_infers_verdict_from_action_dictionary():
    result = infer_type(["allow", "deny", "allow", "drop"], "action", REPO_ROOT)
    assert result.type_name == "verdict"


def test_infers_hash():
    result = infer_type(["d41d8cd98f00b204e9800998ecf8427e"], "filehash", REPO_ROOT)
    assert result.type_name == "hash"


def test_infers_url():
    result = infer_type(
        ["http://example.test/a", "https://example.test/b"], "url", REPO_ROOT
    )
    assert result.type_name == "url"


def test_infers_integer_fallback():
    result = infer_type(
        ["1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12"],
        "count",
        REPO_ROOT,
    )
    assert result.type_name == "integer"


def test_infers_free_text_for_prose():
    result = infer_type(
        ["hello world this is a message", "another totally different sentence here"],
        "msg",
        REPO_ROOT,
    )
    assert result.type_name == "free_text"


def test_empty_values_returns_free_text_zero_confidence():
    result = infer_type([], "x", REPO_ROOT)
    assert result.type_name == "free_text"
    assert result.confidence == 0.0
