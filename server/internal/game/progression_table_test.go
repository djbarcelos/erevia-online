package game

import (
	"os"
	"strings"
	"testing"
)

const (
	progressionDoc   = "../../../docs/progressao.md"
	progressionBegin = "<!-- tabela-teorica:inicio -->\n"
	progressionEnd   = "<!-- tabela-teorica:fim -->"
)

// A tabela teórica do docs/progressao.md é a que sai das fórmulas e dos monstros de hoje.
// Mudou a curva, a XP de um monstro ou as faixas de caça: regere com
// EREVIA_UPDATE_DOCS=1 go test ./internal/game -run TestProgressionDoc
func TestProgressionDocUpToDate(t *testing.T) {
	raw, err := os.ReadFile(progressionDoc)
	if err != nil {
		t.Fatal(err)
	}
	doc := string(raw)
	i, j := strings.Index(doc, progressionBegin), strings.Index(doc, progressionEnd)
	if i < 0 || j < i {
		t.Fatalf("%s sem os marcadores da tabela teórica", progressionDoc)
	}
	want := ProgressionMarkdown(SecondsPerKill)
	if doc[i+len(progressionBegin):j] == want {
		return
	}
	if os.Getenv("EREVIA_UPDATE_DOCS") == "1" {
		doc = doc[:i+len(progressionBegin)] + want + doc[j:]
		if err := os.WriteFile(progressionDoc, []byte(doc), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	t.Fatalf("a tabela de %s está desatualizada: rode EREVIA_UPDATE_DOCS=1 go test ./internal/game -run TestProgressionDoc\n\nquero:\n%s", progressionDoc, want)
}

// A tabela fecha com a curva: cada nível pede exatamente XPToNext, entre abates e missões.
func TestTheoreticalProgressionCoversCurve(t *testing.T) {
	rows := TheoreticalProgression(SecondsPerKill)
	if len(rows) != MaxLevel-1 {
		t.Fatalf("níveis = %d, quero %d", len(rows), MaxLevel-1)
	}
	for _, r := range rows {
		if r.XPPerKill <= 0 || r.Monsters == "" {
			t.Fatalf("nível %d sem caça: %+v", r.Level, r)
		}
		got := float64(r.QuestXP) + float64(r.Kills)*r.XPPerKill
		if got < float64(r.XPToNext) || (r.Kills > 0 && got-r.XPPerKill >= float64(r.XPToNext)) {
			t.Fatalf("nível %d: %d de missões + %d abates × %.0f não fecha %d", r.Level, r.QuestXP, r.Kills, r.XPPerKill, r.XPToNext)
		}
	}
	// No 20 a caça é a dos golens (o grupo mais avançado que cobre o nível).
	if rows[19].Monsters != Monsters["stone_golem"].Name {
		t.Fatalf("nível 20 caça %q", rows[19].Monsters)
	}
}

// Os tempos-alvo do tunnel#149 (~150 h até o 40): cada faixa fica a até 15% do alvo, e
// 25% a 35% da XP de cada faixa vem de missões.
func TestProgressionBandsHitTargets(t *testing.T) {
	bands := ProgressionBands(TheoreticalProgression(SecondsPerKill))
	if len(bands) != 4 || bands[3].To != MaxLevel {
		t.Fatalf("faixas: %+v", bands)
	}
	total := 0.0
	for _, b := range bands {
		total += b.Hours
		if d := b.Hours/b.TargetHours - 1; d < -0.15 || d > 0.15 {
			t.Errorf("%d → %d: %.1f h, alvo %.0f h (%+.0f%%)", b.From, b.To, b.Hours, b.TargetHours, d*100)
		}
		if q := float64(b.QuestXP) / float64(b.XP); q < 0.25 || q > 0.35 {
			t.Errorf("%d → %d: %.0f%% da XP de missões", b.From, b.To, q*100)
		}
	}
	if total < 130 || total > 170 {
		t.Errorf("1 → %d: %.0f h", MaxLevel, total)
	}
}
