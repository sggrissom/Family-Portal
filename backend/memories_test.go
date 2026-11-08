package backend

import (
	"strconv"
	"testing"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

func testMemory(key, kind string, personId int, date string, photoIds ...int) memoryCandidate {
	m := Memory{
		Key:        key,
		Kind:       kind,
		PersonIds:  []int{personId},
		PhotoIds:   photoIds,
		Text:       "something " + key,
		Date:       day(date),
		RecordTime: exactRecordTime(day(date), ""),
	}
	return memoryCandidate{Memory: m, weight: memoryWeight(m)}
}

func memoryKeys(memories []Memory) []string {
	keys := make([]string, len(memories))
	for i, m := range memories {
		keys[i] = m.Key
	}
	return keys
}

func varietyPool() []memoryCandidate {
	var pool []memoryCandidate
	kinds := []string{MemoryMoment, MemoryQuote, MemoryPhoto, MemoryInterview}
	for i := 0; i < 24; i++ {
		date := strconv.Itoa(2015+i%6) + "-0" + strconv.Itoa(1+i%9) + "-10"
		pool = append(pool, testMemory("k"+strconv.Itoa(i), kinds[i%4], 1+i%3, date, 100+i))
	}
	return pool
}

func TestPickMemoriesIsStableForASeed(t *testing.T) {
	pool := varietyPool()
	first := memoryKeys(pickMemories(pool, 7, nil, 10))
	again := memoryKeys(pickMemories(pool, 7, nil, 10))
	other := memoryKeys(pickMemories(pool, 8, nil, 10))
	if len(first) != 10 {
		t.Fatalf("picked %d, want 10", len(first))
	}
	for i := range first {
		if first[i] != again[i] {
			t.Fatalf("same seed picked %v then %v", first, again)
		}
	}
	same := true
	for i := range first {
		same = same && first[i] == other[i]
	}
	if same {
		t.Errorf("seeds 7 and 8 picked the same order %v", first)
	}
}

func TestPickMemoriesVariesNeighbours(t *testing.T) {
	picked := pickMemories(varietyPool(), 3, nil, 8)
	for i := 1; i < len(picked); i++ {
		prev, cur := picked[i-1], picked[i]
		if prev.Kind == cur.Kind {
			t.Errorf("%s and %s are both %s", prev.Key, cur.Key, cur.Kind)
		}
		if sharesPerson(prev.PersonIds, cur.PersonIds) {
			t.Errorf("%s and %s are about the same person", prev.Key, cur.Key)
		}
		if memoryYear(prev) == memoryYear(cur) {
			t.Errorf("%s and %s are from the same year", prev.Key, cur.Key)
		}
	}
}

func TestPickMemoriesNeverRepeatsAnImage(t *testing.T) {
	pool := []memoryCandidate{
		testMemory("a", MemoryMoment, 1, "2018-03-01", 50),
		testMemory("b", MemoryArtwork, 2, "2019-03-01", 50),
		testMemory("c", MemoryQuote, 3, "2020-03-01"),
	}
	picked := pickMemories(pool, 1, nil, 10)
	shown := 0
	for _, m := range picked {
		if containsInt(m.PhotoIds, 50) {
			shown++
		}
	}
	if shown != 1 || len(picked) != 2 {
		t.Fatalf("picked %v; want photo 50 once and two memories", memoryKeys(picked))
	}
}

func TestPickMemoriesNeverRepeatsAText(t *testing.T) {
	first := testMemory("a", MemoryQuote, 1, "2018-03-01")
	second := testMemory("b", MemoryQuote, 2, "2019-03-01")
	first.Text, second.Text = "Why is the moon following us?", "why is the moon following us? "
	if picked := pickMemories([]memoryCandidate{first, second}, 1, nil, 5); len(picked) != 1 {
		t.Fatalf("picked %v, want one of the two identical quotes", memoryKeys(picked))
	}
}

func TestPickMemoriesPutsSeenLast(t *testing.T) {
	pool := varietyPool()
	var seen []string
	for _, c := range pool[:20] {
		seen = append(seen, c.Key)
	}
	picked := pickMemories(pool, 5, seen, 4)
	for _, m := range picked {
		for _, key := range seen {
			if m.Key == key {
				t.Fatalf("picked seen memory %s while unseen ones were left", key)
			}
		}
	}
}

func TestPickMemoriesFallsBackWhenVarietyRunsOut(t *testing.T) {
	pool := []memoryCandidate{
		testMemory("a", MemoryQuote, 1, "2018-03-01"),
		testMemory("b", MemoryQuote, 1, "2018-04-01"),
		testMemory("c", MemoryQuote, 1, "2018-05-01"),
	}
	if picked := pickMemories(pool, 2, nil, 5); len(picked) != 3 {
		t.Fatalf("picked %v, want all three", memoryKeys(picked))
	}
}

func TestMemoryWeightPrefersPhotosAndContext(t *testing.T) {
	bare := Memory{Kind: MemoryMoment, Text: "Walked"}
	rich := Memory{Kind: MemoryMoment, Text: "Walked", PhotoIds: []int{3}, Context: "At grandma's"}
	if memoryWeight(rich) <= memoryWeight(bare) {
		t.Errorf("weight with photo and context %v <= bare %v", memoryWeight(rich), memoryWeight(bare))
	}
	if memoryWeight(Memory{Kind: MemoryPhoto}) >= memoryWeight(Memory{Kind: MemoryQuote}) {
		t.Error("a bare photo outweighs a quote")
	}
}

func TestRecentlyAddedKeepsOnlyOlderRecordsEnteredLately(t *testing.T) {
	today := day("2026-11-08")
	entered := func(c memoryCandidate, created string) memoryCandidate {
		c.CreatedAt = day(created)
		return c
	}
	approx := testMemory("approx", MemoryMoment, 1, "2019-06-01")
	approx.RecordTime = RecordTime{Meaning: MeaningOccurrence, Basis: BasisCalendar, Precision: PrecisionYear, From: "2019"}
	approx.CreatedAt = day("2026-11-05")

	added := recentlyAdded([]memoryCandidate{
		entered(testMemory("old-new", MemoryMoment, 1, "2019-06-01"), "2026-11-07"),
		entered(testMemory("old-older", MemoryMoment, 1, "2018-06-01"), "2026-10-01"),
		entered(testMemory("this-week", MemoryMoment, 1, "2026-11-06"), "2026-11-06"),
		approx,
	}, today)

	keys := memoryKeys(added)
	if len(keys) != 2 || keys[0] != "old-new" || keys[1] != "approx" {
		t.Fatalf("recentlyAdded = %v, want [old-new approx]", keys)
	}
}

func TestGetMemoriesStaysInsideTheFamily(t *testing.T) {
	fx, cleanup := setupIsolationFixture(t)
	defer cleanup()

	call := func(user User, req GetMemoriesRequest) GetMemoriesResponse {
		t.Helper()
		token, err := generateJwtTokenString(user)
		if err != nil {
			t.Fatalf("generateJwtTokenString() error = %v", err)
		}
		var resp GetMemoriesResponse
		vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
			resp, err = GetMemories(&vbeam.Context{Tx: tx, Token: token}, req)
		})
		if err != nil {
			t.Fatalf("GetMemories() error = %v", err)
		}
		return resp
	}

	owner := call(fx.owner, GetMemoriesRequest{Today: "2026-11-08", Seed: 1})
	found := false
	for _, m := range owner.Memories {
		found = found || containsInt(m.PersonIds, fx.person.Id)
	}
	if !found {
		t.Errorf("Memories = %+v, want the fixture child's 2024 milestone", owner.Memories)
	}

	quotes := call(fx.owner, GetMemoriesRequest{Today: "2026-11-08", Seed: 1, Kinds: []string{MemoryQuote}})
	for _, m := range quotes.Memories {
		if m.Kind != MemoryQuote {
			t.Errorf("asked for quotes, got a %s", m.Kind)
		}
	}

	outsider := call(fx.outsider, GetMemoriesRequest{Today: "2026-11-08", Seed: 1, RecentlyAdded: true})
	for _, m := range append(outsider.Memories, outsider.RecentlyAdded...) {
		if containsInt(m.PersonIds, fx.person.Id) {
			t.Errorf("an outsider got a memory of another family's child: %+v", m)
		}
	}
}
