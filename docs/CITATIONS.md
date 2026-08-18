# Citations

Every countable claim this record makes about a text somebody else wrote, and
the computation that settles it.

A claim here is one sentence: *this entry says this about that source, and here
is what the source actually says.* The sentence is quoted from
`docs/DECISIONS.md` verbatim; the source is named by the SHA-256 of its bytes,
not by a filename; the computation is a count over a named region. `cmd/cite`
runs them.

```
go run ./cmd/cite             # every citation
go run ./cmd/cite -list       # the catalog and the sources, running nothing
go run ./cmd/cite -run <id>   # one citation
go run ./cmd/cite -sources    # the manifest only, for a machine with no cache
```

It exists because of D68. Four figures in D67 were wrong, all four were found by
the publication gate — days after the entry was permanent — and D68(f) recorded
that nothing checks a prose entry's numbers before it is committed. The
pre-commit gate reads code and `cmd/seam` reads claims about code. This reads
claims about sources.

**Why this is not `cmd/seam`.** `seam`'s oracle is a test: break the behaviour,
assert the check reddens. This one's oracle is arithmetic: compute the number,
compare it to the sentence. `seam` takes seventeen minutes because it samples a
control on both sides of every mutation, and it therefore cannot run in the
commit hook. This takes milliseconds and belongs there. Two tools rather than
one flag, because the moment the fast check lives inside the slow one it stops
running at the moment it is for.

## What is in a block

**`kind: source`** declares an artifact. `sha256` is the address of the
extracted text and is the only name the catalog uses for it — the cache stores
each source at `<sha256>.txt`, so "which `rqgm.txt`" is not a question anybody
can get wrong. `origin` is where the PDF came from, `pdf` is its own address,
and `extract` is the recipe that turns one into the other. The recipe is prose
and nothing checks it; the two addresses are what make it checkable by hand.

**`kind: count`** asserts that a literal occurs a stated number of times.
`needle` is the string; `from` and `to` delimit a region, each of which must
occur exactly once, and absent them the region is the whole text; `expect` is
the number the sentence states.

**Two keys say how to read the text, and neither has a default.** `fold`
ignores case: `"erasure"` in RQGM is 27 times case-sensitive and 28 times
folded. `flatten` joins the extractor's lines before counting: `"selective
erasure"` is 9 times as extracted and 10 times joined, because one occurrence
is split across a column break. Each was found by writing this file, both times
against a figure the record already carried, and each is a whole number the
record states without saying which question it answers. A count with no case
rule and no line rule is four claims wearing one number, so the block says
both, every time.

**`kind: quotation`** asserts that the source says the words the entry puts in
its mouth. `needle` must occur exactly once. `then` is what the source says
*next*, up to the end of the sentence the quotation was cut out of — required,
always, and empty only when the quotation ends the sentence itself. That key is
the whole point of this kind and it is argued below.

**Every block carries `entry` and `sentence`.** `entry` is the clause, `D68(a)`;
`sentence` is the entry's own words, whitespace-collapsed. The tool requires the
sentence to occur exactly once in that clause, and requires the figure the block
expects to appear in the sentence — as digits, or as the English word for it
where the entry spells small numbers out. **That is what makes this more than a
second copy of a number.** A wrong figure cannot be silenced by editing the
block: change `expect` and the sentence no longer states it, and the block fails
`unquoted` instead. The only way to make a block agree is to fix the record —
which in an append-only log means appending a correction and pointing the
citation at it, which is what D67 and D68 already did by hand.

**`verdict`** is what a block says it will come back as, defaulting to `agrees`,
and the gate is an equality against it in either direction — `cmd/seam`'s rule,
for `cmd/seam`'s reason. It is what lets a wrong figure stay in the catalog as a
wrong figure. D67's four are here, declared `disagrees`, each naming the D68
clause that corrects it. That is not a suppression: the day somebody edits D67 —
which the log forbids — or the day a source moves under a citation, the block
stops being where it says it is and the gate fires.

**The nearest thing here to a check that enforces a bug** (the shape D52(c)
names) is exactly those `disagrees` blocks: they require a false sentence to
remain in D67. It is the right requirement and not that defect, because
`docs/DECISIONS.md` is append-only by decision — the sentence *must* remain, and
a checker that demanded otherwise would be demanding the log be falsified. The
argument is written here rather than assumed, because a reader meeting a block
that requires a disagreement deserves to see it made.

## What it cannot reach

**It does not check that the needle is what the sentence means.** A block
counting `erasure` under a sentence about `Selective Erasure` computes 28 and
agrees, and the sentence stays wrong. The binding above narrows this — the
needle usually appears in the sentence too — but nothing forces it, because a
sentence often names its subject one clause earlier than it states the number.
D67(i) is that exact shape: the string is in one sentence and the count in the
next. **The defence is that the block prints its needle beside the sentence, so
the mismatch is visible to a reader in one line rather than buried in a PDF.**

**It does not check inference.** "The survey disqualifies its own figure" is not
a computation, and no format will make it one. What the `then` key does is
narrower and worth stating exactly: a quotation cut mid-sentence has to declare
where the source's sentence actually ends, so an author who believes a clause
stands alone has to write down the clause that follows it. D67(b) quoted *"the
supplemental harvest is recency-biased by construction"* as the survey
undermining itself; the same sentence continues *"(growth statistics in Figure 6
therefore use the seed corpus only)"*, which is the survey guarding it. Nobody
who typed that continuation would have written the gloss. **The mechanism
catches the truncation, not the misreading**, and the two are not the same
thing: a determined author can write the continuation out and still draw the
opposite conclusion, and nothing here fails.

**It does not check an argument.** "This paper is more serious prior art than
that one" has no number in it and does not belong in this file.

**Only what is cited.** A figure nobody wrote a block for is exactly as
unchecked as it was before this file existed. There is no score and there will
not be one: a percentage goes up when a claim is deleted.

## Where the evidence lives, and why not here

The sources are papers. Three of them are 1.7 MB of extracted text, and the tree
they would live in is public, append-only from its root (D23), and carries no
licence. Vendoring somebody else's paper into a history that cannot be amended
is a one-way door, so the bytes stay out and the addresses come in.

The cache is `$TLDR_SOURCES`, else `$XDG_CACHE_HOME/tldreddit/sources`, else
`~/.cache/tldreddit/sources`, holding one file per source named for its address.
A source that is not there is `evidence-missing` and **fails** — it is not
skipped, because a skip is not a pass and a check that quietly cannot run is the
one this project keeps rebuilding.

The cost is real and is not argued away: a fresh clone cannot run this until
somebody fetches three PDFs and runs the recipe. `-sources` is what a machine
without the cache can still do — it checks that every cited source has a
manifest entry and every manifest entry has an address and a recipe, which is
not the real check and does not pretend to be.

Filling the cache, which is the whole of the setup:

```sh
mkdir -p "${TLDR_SOURCES:-$HOME/.cache/tldreddit/sources}"
curl -Lo /tmp/p.pdf https://arxiv.org/pdf/2606.26294v2      # and the other two
python3 -c 'import pypdf,hashlib,os,sys
b="\n".join(p.extract_text() for p in pypdf.PdfReader(sys.argv[1]).pages).encode()
d=os.environ.get("TLDR_SOURCES", os.path.expanduser("~/.cache/tldreddit/sources"))
open(f"{d}/{hashlib.sha256(b).hexdigest()}.txt","wb").write(b)' /tmp/p.pdf
```

The file lands under its own address, so a wrong PDF or a different `pypdf`
produces a file no citation names and `evidence-missing` says so. Nothing here
has to be told which paper it just extracted.

**Note what the addresses do not pin.** They pin the text and they pin the PDF.
They say nothing about the extractor, so a future `pypdf` that changes its
whitespace produces a text with a different address and every citation goes
`evidence-missing` — loudly, which is right, but the repair is a new manifest
entry and a re-derivation of every figure taken through it, not a one-line edit.
Measured 2026-08-16: all three reproduce byte-identically under `pypdf 6.13.0`,
which is the only version this has ever been true of.

---

## The sources

Measured 2026-08-16: each of the three extracts byte-identically from its PDF
under the recipe below, which is why the recipe is worth writing down rather
than gesturing at. The two NUL bytes D68(b) found in the DGM text and the five
in RQGM's are inside these addresses — they are what the extractor produced, and
a version that cleaned them up would produce a different address and a different
answer to every count here.

```cite
kind: source
id: rqgm
sha256: d1e4473ca0f8cf46edd8cb1a2e3a0f8da1a36a67ba5cfa4bcfa6305bc31190b9
bytes: 135985
origin: https://arxiv.org/abs/2606.26294v2
pdf: 82a2e260eb119d983542489100db195398128976fa20c97838e11d25b80b84f0
extract: pypdf 6.13.0 — "\n".join(p.extract_text() for p in PdfReader(f).pages), encoded UTF-8
```

```cite
kind: source
id: dgm
sha256: e7ebc4caad773c543dffd698877b5f5fdf303621e5bba259002bc7d30623b7ee
bytes: 232219
origin: https://arxiv.org/abs/2505.22954
pdf: 13ff4abe0c7ad4a7dd3b4876d19a8bf940e39e70dabbf06065aa774a6c3457de
extract: pypdf 6.13.0 — "\n".join(p.extract_text() for p in PdfReader(f).pages), encoded UTF-8
```

```cite
kind: source
id: rsi-survey
sha256: 6ab6ee72fdc9efa798f32a796bdeb600ea41b416ff3062ce74e29c292b2ffe4e
bytes: 138038
origin: https://arxiv.org/abs/2607.07663v1
pdf: 0cc637f5492ed565fc9f80fa9a1b5772d3e3743a13ef5bc294c0c081b7142f93
extract: pypdf 6.13.0 — "\n".join(p.extract_text() for p in PdfReader(f).pages), encoded UTF-8
```

---

## A count attached to the wrong string

D67(i)'s subject is a tool that returned zero and was nearly believed, and the
sentence that states the correction carries a number nobody re-derived. The
string it names is `Selective Erasure` and the count it gives is the count of
`erasure`. Off by a factor of three, in the clause about not trusting a count.

The block below is the sentence as D67 published it, and it disagrees. D68(a) is
the correction and its three bullets are the three blocks after it — which is
what makes the pair legible: one figure, two entries, and the tool prints both.

```cite
kind: count
id: d67i-selective-erasure
entry: D67(i)
sentence: Python's `str.count` showed 28 occurrences.
source: rqgm
needle: Selective Erasure
fold: false
flatten: false
expect: 28
verdict: disagrees
```

```cite
kind: count
id: d68a-phrase-cased
entry: D68(a)
sentence: - `"Selective Erasure"`, case-sensitive: **0**
source: rqgm
needle: Selective Erasure
fold: false
flatten: false
expect: 0
```

```cite
kind: count
id: d68a-phrase-folded
entry: D68(a)
sentence: - `"selective erasure"`, case-insensitive: **9**
source: rqgm
needle: selective erasure
fold: true
flatten: false
expect: 9
```

**A finding this file produced rather than recorded, and it is the CEO's to
rule on.** The block above declares `flatten: false` and agrees at 9. Joined,
the same phrase is in RQGM **ten** times — one occurrence is `selective` at the
end of a line and `erasure` at the start of the next, which the extractor's
column break made and the paper did not. So D68(a)'s middle bullet is exactly
right about the file and arguably wrong about the paper, and the entry does not
say which of the two it is answering. Recorded here rather than corrected:
`docs/DECISIONS.md` is append-only and a correction is an entry, not an edit,
and the seat that found this is not the seat that rules on it.

D68(a)'s third bullet gives no case rule where its first two do, and the answer
depends on it: 27 unfolded, 28 folded. The block below reads it as folded,
because that is the number the bullet states — and the fact that reading it the
other way makes the bullet wrong by one is the reason `fold` has no default.

```cite
kind: count
id: d68a-word-folded
entry: D68(a)
sentence: - `"erasure"`: **28**
source: rqgm
needle: erasure
fold: true
flatten: false
expect: 28
```

## A count over a delimited region

D67(c) counts a word inside RQGM's abstract, not inside the file. The region has
to be nameable or the claim is not the claim: `human` is 25 times in the whole
paper and twice in the abstract, and only the second number is what the sentence
is about.

`from` and `to` each occur exactly once in the source, which the tool checks
before it counts anything — a delimiter that matched twice would silently pick a
region nobody chose, which is `cmd/seam`'s `ambiguous-anchor` finding in a
different file.

```cite
kind: count
id: d67c-human-in-abstract
entry: D67(c)
sentence: The word "human" appears in that abstract once, as a comparison baseline.
source: rqgm
from: Abstract.
to: Keywords:
needle: human
fold: true
flatten: false
expect: 1
verdict: disagrees
```

```cite
kind: count
id: d68c-human-in-abstract
entry: D68(c)
sentence: It appears **twice**, both as comparison baselines:
source: rqgm
from: Abstract.
to: Keywords:
needle: human
fold: true
flatten: false
expect: 2
```

## A figure quoted from a table — the control

D67(a) reproduces DGM's Table 1 as four rows of two figures. They are right, and
they are here so that the three disagreements above are a measurement rather
than a tool that fails on everything. Each block asserts that the paper's own
table line occurs once inside the table's region, and binds the two figures on
it to the markdown row D67(a) prints.

A row is a count with `expect: 1` rather than a kind of its own. The extraction
puts each row on its own line, so the line *is* the assertion: change a digit in
the entry and the line is not in the paper.

```cite
kind: count
id: d67a-dgm
entry: D67(a)
sentence: | DGM | 50.0% | 38.0% |
source: dgm
from: Table 1: Comparison of DGM
to: A.4 ADDITIONAL
needle: DGM 50.0% 38.0%
fold: false
flatten: false
expect: 1
figures: 50.0%, 38.0%
```

```cite
kind: count
id: d67a-dgm-no-exploration
entry: D67(a)
sentence: | DGM w/o Open-ended exploration | 23.0% | 14.0% |
source: dgm
from: Table 1: Comparison of DGM
to: A.4 ADDITIONAL
needle: DGM w/o Open-ended exploration 23.0% 14.0%
fold: false
flatten: false
expect: 1
figures: 23.0%, 14.0%
```

```cite
kind: count
id: d67a-dgm-no-self-improve
entry: D67(a)
sentence: | DGM w/o Self-improve | 39.0% | 28.0% |
source: dgm
from: Table 1: Comparison of DGM
to: A.4 ADDITIONAL
needle: DGM w/o Self-improve 39.0% 28.0%
fold: false
flatten: false
expect: 1
figures: 39.0%, 28.0%
```

```cite
kind: count
id: d67a-dgm-greedy
entry: D67(a)
sentence: | DGM Greedy | 39.7% | 30.0% |
source: dgm
from: Table 1: Comparison of DGM
to: A.4 ADDITIONAL
needle: DGM Greedy 39.7% 30.0%
fold: false
flatten: false
expect: 1
figures: 39.7%, 30.0%
```

## A quotation truncated where the source keeps going

D67(b) quotes the RSI survey against its own meaning. The quotation is verbatim
and the claim built on it is false, which is why this is the hardest of the four
and why no arithmetic settles it.

What is settleable is where the source's sentence ends. The block below is
D67(b)'s claim written honestly: it asserts the quoted words are in the survey —
they are — and declares, by leaving `then` empty, that the sentence ends there.
It does not. The tool prints what follows, and what follows is the survey
guarding the figure D67(b) said it was disqualifying.

```cite
kind: quotation
id: d67b-recency-caveat
entry: D67(b)
sentence: The RSI survey (`2607.07663`) supplies the sharpest available line and disqualifies its own headline figure in the same breath — it reports quarterly growth to ~500 papers in 2026 Q2, then notes "the supplemental harvest is recency-biased by construction."
source: rsi-survey
needle: the supplemental harvest is recency-biased by construction
then:
verdict: truncated
```

```cite
kind: quotation
id: d68e-recency-caveat
entry: D68(e)
sentence: D67(b) says the survey "disqualifies its own headline figure in the same breath," quoting "the supplemental harvest is recency-biased by construction."
source: rsi-survey
needle: the supplemental harvest is recency-biased by construction
then: (growth statistics in Figure 6 therefore use the seed corpus only).
```

D67(a)'s quotation of DGM's own gloss is the control for this kind, and it is a
better control than an untruncated quotation would be: it is *also* cut short of
the sentence's end, by a cross-reference that changes nothing. The rule fires on
both, the author looks at both, and only one of them is a finding. A rule that
fired only on the dishonest case would be a rule tuned to one example.

Note what the block cannot carry: D67(a) prints the quotation with a closing
period the paper does not have there. The needle is the words, and where the
words stop is the thing under test.

```cite
kind: quotation
id: d67a-dgm-gloss
entry: D67(a)
sentence: The paper's own gloss, quoted: "only the most recent agent is retained, so a poorly performing self-modification makes subsequent improvements harder to achieve."
source: dgm
needle: only the most recent agent is retained, so a poorly performing self-modification makes subsequent improvements harder to achieve
then: (Appendix A.1).
```
