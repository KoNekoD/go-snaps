package diff

import "strconv"

// Tag Codes
const (
	OpEqual int8 = iota
	OpInsert
	OpDelete
	OpReplace
)

type match struct {
	A    int
	B    int
	Size int
}

type OpCode struct {
	Tag int8
	I1  int
	I2  int
	J1  int
	J2  int
}

// FormatRangeUnified converts range to the "ed" format.
func FormatRangeUnified(start, stop int) string {
	// Per the diff spec at http://www.unix.org/single_unix_specification/
	beginning := start + 1 // lines start numbering with one
	length := stop - start

	if length == 1 {
		return strconv.Itoa(beginning)
	}
	if length == 0 {
		beginning--
	}

	return strconv.Itoa(beginning) + "," + strconv.Itoa(length)
}

// SequenceMatcher compares sequence of strings. The basic
// algorithm predates, and is a little fancier than, an algorithm
// published in the late 1980's by Ratcliff and Obershelp under the
// hyperbolic name "gestalt pattern matching".  The basic idea is to find
// the longest contiguous matching subsequence that contains no "junk"
// elements (R-O doesn't address junk).  The same idea is then applied
// recursively to the pieces of the sequences to the left and to the right
// of the matching subsequence.  This does not yield minimal edit
// sequences, but does tend to yield matches that "look right" to people.
//
// SequenceMatcher tries to compute a "human-friendly diff" between two
// sequences.  Unlike e.g. UNIX(tm) diff, the fundamental notion is the
// longest *contiguous* & junk-free matching subsequence.  That's what
// catches peoples' eyes.  The Windows(tm) windiff has another interesting
// notion, pairing up elements that appear uniquely in each sequence.
// That, and the method here, appear to yield more intuitive difference
// reports than does diff.  This method appears to be the least vulnerable
// to synching up on blocks of "junk lines", though (like blank lines in
// ordinary text files, or maybe "<P>" lines in HTML files).  That may be
// because this is the only method of the 3 that has a *concept* of
// "junk" <wink>.
//
// Timing:  Basic R-O is cubic time worst case and quadratic time expected
// case.  SequenceMatcher is quadratic time for the worst case and has
// expected-case behavior dependent in a complicated way on how many
// elements the sequences have in common; best case time is linear.
type SequenceMatcher struct {
	a              []string
	b              []string
	b2j            map[string][]int
	IsJunk         func(string) bool
	autoJunk       bool
	bJunk          map[string]struct{}
	matchingBlocks []match
	fullBCount     map[string]int
	bPopular       map[string]struct{}
	opCodes        []OpCode
}

func NewMatcher(a, b []string) *SequenceMatcher {
	m := SequenceMatcher{autoJunk: true}
	m.setSeqs(a, b)
	return &m
}

// Set two sequences to be compared.
//
//	s.SetSeqs("abcd", "bcde")
func (m *SequenceMatcher) setSeqs(a, b []string) {
	m.setSeq1(a)
	m.setSeq2(b)
}

// Set the first sequence to be compared.
//
// The second sequence to be compared is not changed.
//
// SequenceMatcher computes and caches detailed information about the
// second sequence, so if you want to compare one sequence S against
// many sequences, use m.setSeq2(S) once and call m.setSeq1(x)
// repeatedly for each of the other sequences.
//
// See also setSeqs() and setSeq2().
func (m *SequenceMatcher) setSeq1(a []string) {
	if &a == &m.a {
		return
	}
	m.a = a
	m.matchingBlocks, m.opCodes = nil, nil
}

// Set the second sequence to be compared.
//
// The first sequence to be compared is not changed.
//
// sequenceMatcher computes and caches detailed information about the
// second sequence, so if you want to compare one sequence S against
// many sequences, use m.setSeq2(S) once and call m.setSeq1(x)
// repeatedly for each of the other sequences.

// See also setSeqs() and setSeq2().
func (m *SequenceMatcher) setSeq2(b []string) {
	if &b == &m.b {
		return
	}
	m.b = b
	m.matchingBlocks, m.opCodes, m.fullBCount = nil, nil, nil
	m.chainB()
}

func (m *SequenceMatcher) chainB() {
	// Populate line -> index mapping
	b2j := map[string][]int{}
	for i, elt := range m.b {
		indices := b2j[elt]
		indices = append(indices, i)
		b2j[elt] = indices
	}

	// Purge junk elements
	m.bJunk = map[string]struct{}{}
	if m.IsJunk != nil {
		junk := m.bJunk
		for elt := range b2j {
			if m.IsJunk(elt) {
				junk[elt] = struct{}{}
			}
		}
		for elt := range junk { // separate loop avoids separate list of keys
			delete(b2j, elt)
		}
	}

	// Purge popular elements that are not junk
	popular := map[string]struct{}{}
	n := len(m.b)
	if m.autoJunk && n >= 200 {
		ntest := n/100 + 1
		for s, indices := range b2j {
			if len(indices) > ntest {
				popular[s] = struct{}{}
			}
		}
		for s := range popular {
			delete(b2j, s)
		}
	}
	m.bPopular = popular
	m.b2j = b2j
}

func (m *SequenceMatcher) isBJunk(s string) bool {
	_, ok := m.bJunk[s]
	return ok
}

// Find longest matching block in a[alo:ahi] and b[blo:bhi].
//
// If IsJunk is not defined:
//
//	Return (i,j,k) such that a[i:i+k] is equal to b[j:j+k], where
//	    alo <= i <= i+k <= ahi
//	    blo <= j <= j+k <= bhi
//	and for all (i',j',k') meeting those conditions,
//	    k >= k'
//	    i <= i'
//	    and if i == i', j <= j'
//
// In other words, of all maximal matching blocks, return one that
// starts earliest in a, and of all those maximal matching blocks that
// start earliest in a, return the one that starts earliest in b.
//
// If IsJunk is defined, first the longest matching block is
// determined as above, but with the additional restriction that no
// junk element appears in the block.  Then that block is extended as
// far as possible by matching (only) junk elements on both sides.  So
// the resulting block never matches on junk except as identical junk
// happens to be adjacent to an "interesting" match.
//
// If no blocks match, return (alo, blo, 0).
func (m *SequenceMatcher) findLongestMatch(alo, ahi, blo, bhi int) match {
	// CAUTION:  stripping common prefix or suffix would be incorrect.
	// E.g.,
	//    ab
	//    acab
	// Longest matching block is "ab", but if common prefix is
	// stripped, it's "a" (tied with "b").  UNIX(tm) diff does so
	// strip, so ends up claiming that ab is changed to acab by
	// inserting "ca" in the middle. That's minimal but unintuitive:
	// "it's obvious" that someone inserted "ac" at the front.
	// Windiff ends up at the same place as diff, but by pairing up
	// the unique 'b's and then matching the first two 'a's.
	besti, bestj, bestsize := alo, blo, 0

	// find longest junk-free match
	// during an iteration of the loop, j2len[j] = length of longest
	// junk-free match ending with a[i-1] and b[j]
	j2len := map[int]int{}
	for i := alo; i != ahi; i++ {
		// look at all instances of a[i] in b; note that because
		// b2j has no junk keys, the loop is skipped if a[i] is junk
		newj2len := map[int]int{}
		for _, j := range m.b2j[m.a[i]] {
			// a[i] matches b[j]
			if j < blo {
				continue
			}
			if j >= bhi {
				break
			}
			k := j2len[j-1] + 1
			newj2len[j] = k
			if k > bestsize {
				besti, bestj, bestsize = i-k+1, j-k+1, k
			}
		}
		j2len = newj2len
	}

	// Extend the best by non-junk elements on each end. In particular,
	// "popular" non-junk elements aren't in b2j, which greatly speeds
	// the inner loop above, but also means "the best" match so far
	// doesn't contain any junk *or* popular non-junk elements.
	for besti > alo && bestj > blo && !m.isBJunk(m.b[bestj-1]) &&
		m.a[besti-1] == m.b[bestj-1] {
		besti, bestj, bestsize = besti-1, bestj-1, bestsize+1
	}
	for besti+bestsize < ahi && bestj+bestsize < bhi &&
		!m.isBJunk(m.b[bestj+bestsize]) &&
		m.a[besti+bestsize] == m.b[bestj+bestsize] {
		bestsize++
	}

	// Now that we have a wholly interesting match (albeit possibly
	// empty!), we may as well suck up the matching junk on each
	// side of it too. Can't think of a good reason not to, and it
	// saves post-processing the (possibly considerable) expense of
	// figuring out what to do with it.  In the case of an empty
	// interesting match, this is clearly the right thing to do,
	// because no other kind of match is possible in the regions.
	for besti > alo && bestj > blo && m.isBJunk(m.b[bestj-1]) &&
		m.a[besti-1] == m.b[bestj-1] {
		besti, bestj, bestsize = besti-1, bestj-1, bestsize+1
	}
	for besti+bestsize < ahi && bestj+bestsize < bhi &&
		m.isBJunk(m.b[bestj+bestsize]) &&
		m.a[besti+bestsize] == m.b[bestj+bestsize] {
		bestsize++
	}

	return match{A: besti, B: bestj, Size: bestsize}
}

// Return list of triples describing matching subsequences.
//
// Each triple is of the form (i, j, n), and means that
// a[i:i+n] == b[j:j+n].  The triples are monotonically increasing in
// i and in j. It's also guaranteed that if (i, j, n) and (i', j', n') are
// adjacent triples in the list, and the second is not the last triple in the
// list, then i+n != i' or j+n != j'. IOW, adjacent triples never describe
// adjacent equal blocks.
//
// The last triple is a dummy, (len(a), len(b), 0), and is the only
// triple with n==0.
func (m *SequenceMatcher) getMatchingBlocks() []match {
	if m.matchingBlocks != nil {
		return m.matchingBlocks
	}

	var matchBlocks func(alo, ahi, blo, bhi int, matched []match) []match
	matchBlocks = func(alo, ahi, blo, bhi int, matched []match) []match {
		match := m.findLongestMatch(alo, ahi, blo, bhi)
		i, j, k := match.A, match.B, match.Size
		if match.Size > 0 {
			if alo < i && blo < j {
				matched = matchBlocks(alo, i, blo, j, matched)
			}
			matched = append(matched, match)
			if i+k < ahi && j+k < bhi {
				matched = matchBlocks(i+k, ahi, j+k, bhi, matched)
			}
		}
		return matched
	}
	matched := matchBlocks(0, len(m.a), 0, len(m.b), nil)

	// It's possible that we have adjacent equal blocks in the
	// matching_blocks list now.
	nonAdjacent := []match{}
	i1, j1, k1 := 0, 0, 0
	for _, b := range matched {
		// Is this block adjacent to i1, j1, k1?
		i2, j2, k2 := b.A, b.B, b.Size
		if i1+k1 == i2 && j1+k1 == j2 {
			// Yes, so collapse them -- this just increases the length of
			// the first block by the length of the second, and the first
			// block so lengthened remains the block to compare against.
			k1 += k2
		} else {
			// Not adjacent.  Remember the first block (k1==0 means it's
			// the dummy we started with), and make the second block the
			// new block to compare against.
			if k1 > 0 {
				nonAdjacent = append(nonAdjacent, match{i1, j1, k1})
			}
			i1, j1, k1 = i2, j2, k2
		}
	}
	if k1 > 0 {
		nonAdjacent = append(nonAdjacent, match{i1, j1, k1})
	}

	nonAdjacent = append(nonAdjacent, match{len(m.a), len(m.b), 0})
	m.matchingBlocks = nonAdjacent
	return m.matchingBlocks
}

// Return list of 5-tuples describing how to turn a into b.
//
// Each tuple is of the form (tag, i1, i2, j1, j2). The first tuple
// has i1 == j1 == 0, and remaining tuples have i1 == the i2 from the
// tuple preceding it, and likewise for j1 == the previous j2.
//
// The tags are characters, with these meanings:
//
// OpReplace (replace):  a[i1:i2] should be replaced by b[j1:j2]
//
// OpDelete (delete):    a[i1:i2] should be deleted, j1==j2 in this case.
//
// OpInsert (insert):    b[j1:j2] should be inserted at a[i1:i1], i1==i2 in this case.
//
// OpEqual (equal):      a[i1:i2] == b[j1:j2]
func (m *SequenceMatcher) getOpCodes() []OpCode {
	if m.opCodes != nil {
		return m.opCodes
	}
	i, j := 0, 0
	matching := m.getMatchingBlocks()
	opCodes := make([]OpCode, 0, len(matching))
	for _, m := range matching {
		//  invariant: we've pumped out correct diffs to change
		//  a[:i] into b[:j], and the next matching block is
		//  a[ai:ai+size] == b[bj:bj+size]. So we need to pump
		//  out a diff to change a[i:ai] into b[j:bj], pump out
		//  the matching block, and move (i,j) beyond the match
		ai, bj, size := m.A, m.B, m.Size
		var tag int8 = 0
		if i < ai && j < bj {
			tag = OpReplace
		} else if i < ai {
			tag = OpDelete
		} else if j < bj {
			tag = OpInsert
		}
		if tag > 0 {
			opCodes = append(opCodes, OpCode{tag, i, ai, j, bj})
		}
		i, j = ai+size, bj+size
		// the list of matching blocks is terminated by a
		// sentinel with size 0
		if size > 0 {
			opCodes = append(opCodes, OpCode{OpEqual, ai, i, bj, j})
		}
	}
	m.opCodes = opCodes
	return m.opCodes
}

// GetGroupedOpCodes Isolate change clusters by eliminating ranges with no changes.
//
// Return a generator of groups with up to n lines of context.
// Each group is in the same format as returned by getOpCodes().
func (m *SequenceMatcher) GetGroupedOpCodes(n int) [][]OpCode {
	if n < 0 {
		n = 3
	}
	codes := m.getOpCodes()
	if len(codes) == 0 {
		codes = []OpCode{{OpEqual, 0, 1, 0, 1}}
	}
	// Fixup leading and trailing groups if they show no changes.
	if codes[0].Tag == OpEqual {
		c := codes[0]
		i1, i2, j1, j2 := c.I1, c.I2, c.J1, c.J2
		codes[0] = OpCode{c.Tag, max(i1, i2-n), i2, max(j1, j2-n), j2}
	}
	if codes[len(codes)-1].Tag == OpEqual {
		c := codes[len(codes)-1]
		i1, i2, j1, j2 := c.I1, c.I2, c.J1, c.J2
		codes[len(codes)-1] = OpCode{c.Tag, i1, min(i2, i1+n), j1, min(j2, j1+n)}
	}
	nn := n + n
	groups := [][]OpCode{}
	group := []OpCode{}
	for _, c := range codes {
		i1, i2, j1, j2 := c.I1, c.I2, c.J1, c.J2
		// End the current group and start a new one whenever
		// there is a large range with no changes.
		if c.Tag == OpEqual && i2-i1 > nn {
			group = append(group, OpCode{
				c.Tag, i1, min(i2, i1+n),
				j1, min(j2, j1+n),
			})
			groups = append(groups, group)
			group = []OpCode{}
			i1, j1 = max(i1, i2-n), max(j1, j2-n)
		}
		group = append(group, OpCode{c.Tag, i1, i2, j1, j2})
	}
	if len(group) > 0 && !(len(group) == 1 && group[0].Tag == OpEqual) {
		groups = append(groups, group)
	}
	return groups
}
