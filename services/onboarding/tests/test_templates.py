from app.templates import mine_templates


def test_mines_a_single_template_for_uniform_lines():
    lines = [f"user{i} logged in from 10.0.0.{i}" for i in range(20)]
    result = mine_templates(lines)
    assert result.total_lines == 20
    assert len(result.clusters) == 1
    assert result.clusters[0].coverage_pct == 100.0
    assert "<*>" in result.clusters[0].template


def test_mines_separate_templates_for_distinct_shapes():
    lines = [f"user{i} logged in from 10.0.0.{i}" for i in range(10)]
    lines += [f"payment {i} processed for order {i * 3}" for i in range(10)]
    result = mine_templates(lines)
    assert result.total_lines == 20
    assert len(result.clusters) >= 2
    assert result.total_coverage_pct == 100.0


def test_empty_sample():
    result = mine_templates([])
    assert result.total_lines == 0
    assert result.clusters == []
