from src.voice.tokenizer import tokenize
from src.voice.ngram import build_segments, generate_ngrams, Segment, NGram


def test_build_segments_no_quantity_creates_one_implicit_segment():
    tokens = tokenize("молоко простоквашино сыр")
    segs = build_segments(tokens)
    assert len(segs) == 1
    assert segs[0].quantity == 1
    assert segs[0].words == ["молоко", "простоквашино", "сыр"]
    assert segs[0].unit is None


def test_build_segments_quantity_starts_new_segment():
    tokens = tokenize("два молока пять огурцов")
    segs = build_segments(tokens)
    assert len(segs) == 2
    assert segs[0].quantity == 2 and segs[0].words == ["молока"]
    assert segs[1].quantity == 5 and segs[1].words == ["огурцов"]


def test_build_segments_implicit_first_segment_when_words_before_quantity():
    tokens = tokenize("молоко два батона")
    segs = build_segments(tokens)
    assert len(segs) == 2
    assert segs[0].quantity == 1 and segs[0].words == ["молоко"]
    assert segs[1].quantity == 2 and segs[1].words == ["батона"]


def test_build_segments_attaches_unit_to_segment():
    tokens = tokenize("два литра молока")
    segs = build_segments(tokens)
    assert len(segs) == 1
    assert segs[0].quantity == 2
    assert segs[0].unit == "л"
    assert segs[0].words == ["молока"]


def test_build_segments_ignores_stopwords_in_words():
    tokens = tokenize("молоко и хлеб")
    segs = build_segments(tokens)
    assert len(segs) == 1
    assert segs[0].words == ["молоко", "хлеб"]


def test_build_segments_skips_empty_segments():
    tokens = tokenize("два пять огурцов")
    segs = build_segments(tokens)
    assert len(segs) == 1
    assert segs[0].quantity == 5 and segs[0].words == ["огурцов"]


def test_generate_ngrams_unigrams_only_for_single_word_segment():
    seg = Segment(quantity=1, unit=None, words=["молоко"])
    ngrams = generate_ngrams(seg, max_n=3)
    assert ngrams == [NGram(text="молоко", start=0, end=1)]


def test_generate_ngrams_creates_all_lengths_up_to_max():
    seg = Segment(quantity=1, unit=None, words=["молоко", "простоквашино"])
    ngrams = generate_ngrams(seg, max_n=3)
    assert NGram(text="молоко", start=0, end=1) in ngrams
    assert NGram(text="простоквашино", start=1, end=2) in ngrams
    assert NGram(text="молоко простоквашино", start=0, end=2) in ngrams
    assert len(ngrams) == 3


def test_generate_ngrams_clamps_n_to_word_count():
    seg = Segment(quantity=1, unit=None, words=["молоко"])
    ngrams = generate_ngrams(seg, max_n=5)
    assert len(ngrams) == 1


def test_generate_ngrams_three_words_max_n_three():
    seg = Segment(quantity=1, unit=None, words=["сыр", "российский", "тёртый"])
    ngrams = generate_ngrams(seg, max_n=3)
    texts = sorted(n.text for n in ngrams)
    assert texts == sorted([
        "сыр", "российский", "тёртый",
        "сыр российский", "российский тёртый",
        "сыр российский тёртый",
    ])
