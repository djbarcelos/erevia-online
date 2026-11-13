package zone

import (
	"context"
	"math"
	"sort"
	"strconv"
	"strings"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// HotbarSize: teclas 1–0.
const HotbarSize = 10

type progressEvent struct {
	id  uint32
	msg any // *pb.AllocateAttrs, *pb.LearnSkills, *pb.SetHotbar, *pb.SpecCmd
}

func (progressEvent) isEvent() {}

// ProgressAction repassa um pedido de progressão (atributos, skills, barra de atalhos).
func (z *Zone) ProgressAction(ctx context.Context, id uint32, msg any) {
	z.post(ctx, progressEvent{id: id, msg: msg})
}

func (z *Zone) onProgress(e progressEvent) {
	p := z.players[e.id]
	if p == nil || !z.spendAction(p) {
		return
	}
	switch m := e.msg.(type) {
	case *pb.AllocateAttrs:
		z.allocateAttrs(p, m.Add)
	case *pb.LearnSkills:
		z.learnSkills(p, m.Ranks)
	case *pb.SetHotbar:
		z.setHotbar(p, m.Slots)
	case *pb.SpecCmd:
		z.onSpecCmd(p, m)
	}
}

// ---------------------------------------------------------------------------------------
// Atributos de combate
// ---------------------------------------------------------------------------------------

// computeStats: classe no nível → atributos (com o bônus da especialização) →
// equipamento → passivas → regeneração (a conta fica em game.CharacterStats, que o
// simulador de balanceamento também usa).
func (z *Zone) computeStats(p *entity, spent game.Attrs) game.Stats {
	return game.CharacterStats(p.pl.class, p.pl.spec, p.level, spent, p.pl.inv.bonus(), p.pl.ranks, p.pl.inv.weaponType())
}

// refreshStats recalcula os atributos (nível, pontos, equipamento ou passivas mudaram).
func (z *Zone) refreshStats(p *entity) {
	p.stats = z.computeStats(p, p.pl.attrs)
	p.hp = math.Min(p.hp, p.stats.MaxHP)
	p.mp = math.Min(p.mp, p.stats.MaxMP)
	p.pl.statsDirty = true
}

func statBlock(s game.Stats) *pb.StatBlock {
	atkSpeed := 0.0
	if s.AtkInterval > 0 {
		atkSpeed = 1 / s.AtkInterval
	}
	return &pb.StatBlock{
		MaxHp: float32(s.MaxHP), MaxMp: float32(s.MaxMP), PAtk: float32(s.PAtk), MAtk: float32(s.MAtk),
		PDef: float32(s.PDef), MDef: float32(s.MDef), Crit: float32(s.Crit),
		Accuracy: float32(s.Accuracy), Evasion: float32(s.Evasion), AtkSpeed: float32(atkSpeed),
		CastSpeed: float32(s.CastSpeed), HpRegen: float32(s.HPRegen), MpRegen: float32(s.MPRegen),
		CritDamage: float32(s.CritDamage), Speed: float32(s.Speed),
	}
}

func attrsPB(a game.Attrs) *pb.Attrs {
	return &pb.Attrs{Str: uint32(a.Str), Dex: uint32(a.Dex), Con: uint32(a.Con), Int: uint32(a.Int), Wis: uint32(a.Wis), Spi: uint32(a.Spi)}
}

func freeAttrPoints(p *entity) int {
	return max(0, game.AttrPointsTotal(p.level)-p.pl.attrs.Sum())
}

func freeSkillPoints(p *entity) int {
	return max(0, game.SkillPointsTotal(p.level)-game.RanksUsed(p.pl.ranks))
}

// progressPB: atributos, ranks e o efeito de +1 ponto em cada atributo (para a prévia).
func (z *Zone) progressPB(p *entity) *pb.Progress {
	out := &pb.Progress{
		Base:            attrsPB(game.ClassBaseAttrs[p.pl.class]),
		Spent:           attrsPB(p.pl.attrs),
		FreeAttrPoints:  uint32(freeAttrPoints(p)),
		FreeSkillPoints: uint32(freeSkillPoints(p)),
		Spec:            p.pl.spec,
	}
	ids := make([]uint32, 0, len(p.pl.ranks))
	for id := range p.pl.ranks {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		if r := p.pl.ranks[id]; r > 0 {
			out.Ranks = append(out.Ranks, &pb.SkillRank{SkillId: id, Rank: uint32(r)})
		}
	}
	now := z.computeStats(p, p.pl.attrs)
	for i := 0; i < 6; i++ {
		next := z.computeStats(p, p.pl.attrs.Add(game.UnitAttr(i)))
		a, b := statBlock(now), statBlock(next)
		out.PerPoint = append(out.PerPoint, &pb.StatBlock{
			MaxHp: b.MaxHp - a.MaxHp, MaxMp: b.MaxMp - a.MaxMp, PAtk: b.PAtk - a.PAtk, MAtk: b.MAtk - a.MAtk,
			PDef: b.PDef - a.PDef, MDef: b.MDef - a.MDef, Crit: b.Crit - a.Crit, Accuracy: b.Accuracy - a.Accuracy,
			Evasion: b.Evasion - a.Evasion, AtkSpeed: b.AtkSpeed - a.AtkSpeed, CastSpeed: b.CastSpeed - a.CastSpeed,
			HpRegen: b.HpRegen - a.HpRegen, MpRegen: b.MpRegen - a.MpRegen, CritDamage: b.CritDamage - a.CritDamage,
		})
	}
	return out
}

func (z *Zone) sendProgress(p *entity) {
	z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_Progress{Progress: z.progressPB(p)}})
}

// ---------------------------------------------------------------------------------------
// Comandos
// ---------------------------------------------------------------------------------------

func (z *Zone) allocateAttrs(p *entity, add *pb.Attrs) {
	if add == nil {
		return
	}
	a := game.Attrs{Str: int(add.Str), Dex: int(add.Dex), Con: int(add.Con), Int: int(add.Int), Wis: int(add.Wis), Spi: int(add.Spi)}
	n := a.Sum()
	if n <= 0 || !a.Valid() {
		return
	}
	if n > freeAttrPoints(p) {
		z.system(p, "Pontos de atributo insuficientes.")
		z.sendProgress(p)
		return
	}
	p.pl.attrs = p.pl.attrs.Add(a)
	z.refreshStats(p)
	z.sendProgress(p)
	z.saveSoon(p)
}

func (z *Zone) learnSkills(p *entity, want []*pb.SkillRank) {
	if len(want) == 0 {
		return
	}
	next := make(map[uint32]int, len(p.pl.ranks)+len(want))
	for id, r := range p.pl.ranks {
		next[id] = r
	}
	for _, w := range want {
		if int(w.Rank) < next[w.SkillId] {
			z.system(p, "Não dá para desaprender uma skill.")
			z.sendProgress(p)
			return
		}
		next[w.SkillId] = int(w.Rank)
	}
	if err := game.ValidateRanks(p.pl.class, p.pl.spec, p.level, next); err != nil {
		z.system(p, "Não foi possível aprender: "+err.Error()+".")
		z.sendProgress(p)
		return
	}
	p.pl.ranks = next
	z.refreshStats(p) // passivas
	z.sendProgress(p)
	z.saveSoon(p)
}

// validHotbarEntry: "" (vazio), "skill:<id>" de uma ativa aprendida da classe, ou
// "item:<id>" de um consumível.
func validHotbarEntry(p *entity, e string) bool {
	switch {
	case e == "":
		return true
	case strings.HasPrefix(e, "skill:"):
		id, err := strconv.ParseUint(e[6:], 10, 32)
		if err != nil {
			return false
		}
		s := game.Skills[uint32(id)]
		return s != nil && s.Class == p.pl.class && (s.Spec == "" || s.Spec == p.pl.spec) && !s.IsPassive() && p.pl.rank(s.ID) > 0
	case strings.HasPrefix(e, "item:"):
		d := game.Items[e[5:]]
		return d != nil && (d.HealHP > 0 || d.HealMP > 0)
	}
	return false
}

func (z *Zone) setHotbar(p *entity, slots []string) {
	if len(slots) > HotbarSize {
		return
	}
	bar := make([]string, HotbarSize)
	copy(bar, slots)
	for _, e := range bar {
		if len(e) > 40 || !validHotbarEntry(p, e) {
			z.sendHotbar(p) // devolve a barra atual (o cliente se corrige)
			return
		}
	}
	p.pl.hotbar = bar
	z.sendHotbar(p)
	z.saveSoon(p)
}

func (z *Zone) sendHotbar(p *entity) {
	z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_Hotbar{Hotbar: &pb.Hotbar{Slots: p.pl.hotbar}}})
}

// DefaultHotbar: skills iniciais nas teclas 1–2 e poções nas 3–6 (como era antes).
func DefaultHotbar(class string) []string {
	bar := make([]string, HotbarSize)
	for i, s := range game.ClassSkills(class) {
		bar[i] = "skill:" + strconv.FormatUint(uint64(s.ID), 10)
	}
	copy(bar[2:], []string{"item:potion_hp_s", "item:potion_mp_s", "item:potion_hp_l", "item:potion_mp_l"})
	return bar
}

// loadProgress normaliza o que veio do banco: árvore vazia (personagem antigo) ganha as
// skills iniciais; ranks inválidos (ou de outro caminho que não spec) são descartados;
// barra vazia ganha o padrão.
func loadProgress(class, spec string, level int, attrs game.Attrs, ranks map[uint32]int, hotbar []string) (game.Attrs, map[uint32]int, []string) {
	if !attrs.Valid() || attrs.Sum() > game.AttrPointsTotal(level) {
		attrs = game.Attrs{}
	}
	clean := map[uint32]int{}
	for id, r := range ranks {
		if s := game.Skills[id]; s != nil && s.Class == class && (s.Spec == "" || s.Spec == spec) && r > 0 {
			clean[id] = min(r, s.MaxRank)
		}
	}
	for id, r := range game.StarterRanks(class) {
		if clean[id] < r {
			clean[id] = r
		}
	}
	if game.ValidateRanks(class, spec, level, clean) != nil {
		clean = game.StarterRanks(class)
	}
	if len(hotbar) == 0 {
		hotbar = DefaultHotbar(class)
	}
	bar := make([]string, HotbarSize)
	copy(bar, hotbar)
	return attrs, clean, bar
}
