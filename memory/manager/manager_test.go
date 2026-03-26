package manager

import (
	"context"
	"testing"
	"time"
	"xgc-agent/memory"
	type_ "xgc-agent/memory/type_"
	"xgc-agent/message"
)

type stubShortTerm struct {
	snapshot []memory.MemoryItem
	added    []memory.MemoryItem
}

func (s *stubShortTerm) Add(ctx context.Context, items ...memory.MemoryItem) error {
	s.added = append(s.added, items...)
	return nil
}
func (s *stubShortTerm) Update(ctx context.Context, items ...memory.MemoryItem) error { return nil }
func (s *stubShortTerm) Get(ctx context.Context, id string) (memory.MemoryItem, error) {
	return memory.MemoryItem{}, memory.ErrNotFound
}
func (s *stubShortTerm) List(ctx context.Context, opts memory.ListOptions) ([]memory.MemoryItem, error) {
	return s.snapshot, nil
}
func (s *stubShortTerm) Delete(ctx context.Context, id string) error { return nil }
func (s *stubShortTerm) Clear(ctx context.Context) error             { return nil }
func (s *stubShortTerm) Search(ctx context.Context, query string, limit int) ([]memory.MemoryItem, error) {
	return nil, memory.ErrSearchNotSupported
}
func (s *stubShortTerm) SessionID() string { return "sess-1" }
func (s *stubShortTerm) Snapshot(ctx context.Context) ([]memory.MemoryItem, error) {
	return s.snapshot, nil
}
func (s *stubShortTerm) Len() int { return len(s.snapshot) }

var _ type_.BaseShortTermMemory = (*stubShortTerm)(nil)

type stubEpisodicMemory struct {
	added   []memory.MemoryItem
	results []memory.MemoryItem
}

func (s *stubEpisodicMemory) Add(ctx context.Context, items ...memory.MemoryItem) error {
	s.added = append(s.added, items...)
	return nil
}
func (s *stubEpisodicMemory) AddEpisode(ctx context.Context, items ...memory.MemoryItem) error {
	return s.Add(ctx, items...)
}
func (s *stubEpisodicMemory) Update(ctx context.Context, items ...memory.MemoryItem) error {
	return nil
}
func (s *stubEpisodicMemory) Get(ctx context.Context, id string) (memory.MemoryItem, error) {
	return memory.MemoryItem{}, memory.ErrNotFound
}
func (s *stubEpisodicMemory) List(ctx context.Context, opts memory.ListOptions) ([]memory.MemoryItem, error) {
	return nil, nil
}
func (s *stubEpisodicMemory) ListByTimeRange(ctx context.Context, since, until time.Time, limit int) ([]memory.MemoryItem, error) {
	return nil, nil
}
func (s *stubEpisodicMemory) Delete(ctx context.Context, id string) error { return nil }
func (s *stubEpisodicMemory) Clear(ctx context.Context) error             { return nil }
func (s *stubEpisodicMemory) Search(ctx context.Context, query string, limit int) ([]memory.MemoryItem, error) {
	return s.results, nil
}

var _ type_.EpisodicMemory = (*stubEpisodicMemory)(nil)

type stubSemanticMemory struct {
	added   []memory.MemoryItem
	results []memory.MemoryItem
}

func (s *stubSemanticMemory) Add(ctx context.Context, items ...memory.MemoryItem) error {
	s.added = append(s.added, items...)
	return nil
}
func (s *stubSemanticMemory) AddKnowledge(ctx context.Context, items ...memory.MemoryItem) error {
	return s.Add(ctx, items...)
}
func (s *stubSemanticMemory) Update(ctx context.Context, items ...memory.MemoryItem) error {
	return nil
}
func (s *stubSemanticMemory) Get(ctx context.Context, id string) (memory.MemoryItem, error) {
	return memory.MemoryItem{}, memory.ErrNotFound
}
func (s *stubSemanticMemory) List(ctx context.Context, opts memory.ListOptions) ([]memory.MemoryItem, error) {
	return nil, nil
}
func (s *stubSemanticMemory) Delete(ctx context.Context, id string) error { return nil }
func (s *stubSemanticMemory) Clear(ctx context.Context) error             { return nil }
func (s *stubSemanticMemory) Search(ctx context.Context, query string, limit int) ([]memory.MemoryItem, error) {
	return s.results, nil
}
func (s *stubSemanticMemory) SearchByTopic(ctx context.Context, topic string, limit int) ([]memory.MemoryItem, error) {
	return nil, nil
}

var _ type_.SemanticMemory = (*stubSemanticMemory)(nil)

type stubPerceptualMemory struct{}

func (s *stubPerceptualMemory) Add(ctx context.Context, items ...memory.MemoryItem) error { return nil }
func (s *stubPerceptualMemory) AddPerception(ctx context.Context, items ...memory.MemoryItem) error {
	return nil
}
func (s *stubPerceptualMemory) Update(ctx context.Context, items ...memory.MemoryItem) error {
	return nil
}
func (s *stubPerceptualMemory) Get(ctx context.Context, id string) (memory.MemoryItem, error) {
	return memory.MemoryItem{}, memory.ErrNotFound
}
func (s *stubPerceptualMemory) List(ctx context.Context, opts memory.ListOptions) ([]memory.MemoryItem, error) {
	return nil, nil
}
func (s *stubPerceptualMemory) ListByModality(ctx context.Context, modality string, limit int) ([]memory.MemoryItem, error) {
	return nil, nil
}
func (s *stubPerceptualMemory) Delete(ctx context.Context, id string) error { return nil }
func (s *stubPerceptualMemory) Clear(ctx context.Context) error             { return nil }
func (s *stubPerceptualMemory) Search(ctx context.Context, query string, limit int) ([]memory.MemoryItem, error) {
	return nil, memory.ErrSearchNotSupported
}

var _ type_.PerceptualMemory = (*stubPerceptualMemory)(nil)

type stubPromotionAnalyzer struct {
	decision PromotionDecision
}

func (s *stubPromotionAnalyzer) Analyze(ctx context.Context, messages []message.Message) (PromotionDecision, error) {
	return s.decision, nil
}

func TestMemoryManagerPromoteRoutesEpisodic(t *testing.T) {
	st := &stubShortTerm{snapshot: []memory.MemoryItem{{
		UserID: "u1",
		Memory: &memory.Memory{Content: "We fixed production incident", Metadata: map[string]any{"role": "user"}},
	}}}
	epi := &stubEpisodicMemory{}
	sem := &stubSemanticMemory{}
	lt := type_.NewLongTermMemory(epi, sem, &stubPerceptualMemory{})

	mgr := NewMemoryManager(
		WithShortTerm(st),
		WithLongTerm(lt),
		WithPromotionAnalyzer(&stubPromotionAnalyzer{decision: PromotionDecision{
			Type:    memory.MemoryEpisodic,
			Summary: "incident handled",
			Episodic: EpisodicPromotionData{
				EventText:    "Fixed prod incident",
				Participants: []string{"user", "assistant"},
				OccurredAt:   "2026-03-14T10:00:00Z",
				Outcome:      "service restored",
				Completed:    true,
			},
		}}),
	)

	if err := mgr.Promote(context.Background(), "s1"); err != nil {
		t.Fatalf("Promote() error = %v", err)
	}
	if len(epi.added) != 1 {
		t.Fatalf("episodic added count = %d, want 1", len(epi.added))
	}
	added := epi.added[0]
	if added.Type != memory.MemoryEpisodic {
		t.Fatalf("added type = %s, want %s", added.Type, memory.MemoryEpisodic)
	}
	if added.Memory == nil || added.Memory.Metadata["event_text"] == "" {
		t.Fatalf("episodic metadata missing event_text: %+v", added.Memory)
	}
}

func TestMemoryManagerPromoteRoutesSemantic(t *testing.T) {
	st := &stubShortTerm{snapshot: []memory.MemoryItem{{
		UserID: "u1",
		Memory: &memory.Memory{Content: "I prefer concise responses", Metadata: map[string]any{"role": "user"}},
	}}}
	epi := &stubEpisodicMemory{}
	sem := &stubSemanticMemory{}
	lt := type_.NewLongTermMemory(epi, sem, &stubPerceptualMemory{})

	mgr := NewMemoryManager(
		WithShortTerm(st),
		WithLongTerm(lt),
		WithPromotionAnalyzer(&stubPromotionAnalyzer{decision: PromotionDecision{
			Type:    memory.MemorySemantic,
			Summary: "User prefers concise responses",
			Semantic: SemanticPromotionData{
				Knowledge: "User prefers concise responses",
				Entities:  []map[string]any{{"entity_id": "user_pref", "name": "concise responses", "type": "PREFERENCE"}},
				Relations: []map[string]any{{"from": "user_pref", "to": "assistant_policy", "type": "GUIDES"}},
			},
		}}),
	)

	if err := mgr.Promote(context.Background(), "s1"); err != nil {
		t.Fatalf("Promote() error = %v", err)
	}
	if len(sem.added) != 1 {
		t.Fatalf("semantic added count = %d, want 1", len(sem.added))
	}
	added := sem.added[0]
	if added.Type != memory.MemorySemantic {
		t.Fatalf("added type = %s, want %s", added.Type, memory.MemorySemantic)
	}
	if added.Memory == nil {
		t.Fatal("semantic memory payload is nil")
	}
	if _, ok := added.Memory.Metadata["entities"]; !ok {
		t.Fatalf("semantic metadata missing entities: %+v", added.Memory.Metadata)
	}
	if _, ok := added.Memory.Metadata["relations"]; !ok {
		t.Fatalf("semantic metadata missing relations: %+v", added.Memory.Metadata)
	}
}

func TestMemoryManagerRetrieveLayeredAndMixed(t *testing.T) {
	epi := &stubEpisodicMemory{results: []memory.MemoryItem{{MemoryID: "e1", Score: 0.7, Type: memory.MemoryEpisodic}}}
	sem := &stubSemanticMemory{results: []memory.MemoryItem{{MemoryID: "s1", Score: 0.9, Type: memory.MemorySemantic}}}
	lt := type_.NewLongTermMemory(epi, sem, &stubPerceptualMemory{})
	mgr := NewMemoryManager(WithLongTerm(lt))

	layered, err := mgr.RetrieveLayered(context.Background(), "query", 5)
	if err != nil {
		t.Fatalf("RetrieveLayered() error = %v", err)
	}
	if len(layered.Episodic) != 1 || len(layered.Semantic) != 1 {
		t.Fatalf("unexpected layered result: %+v", layered)
	}

	mixed, err := mgr.Retrieve(context.Background(), "query", 1)
	if err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}
	if len(mixed) != 1 {
		t.Fatalf("mixed len = %d, want 1", len(mixed))
	}
	if mixed[0].MemoryID != "s1" {
		t.Fatalf("top mixed memory = %s, want s1", mixed[0].MemoryID)
	}
}
