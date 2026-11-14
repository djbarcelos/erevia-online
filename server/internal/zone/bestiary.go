package zone

import (
	"fmt"
	"maps"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// Bestiário (game/bestiary.go): a contagem de abates por tipo de monstro, os marcos, as
// famílias e o bônus de dano do último marco.
//
// Quem conta o abate: os mesmos que levam a XP do monstro (creditGroup + eligible: o grupo,
// ou o jogador sozinho, que causou mais dano, vivos a até partyRange e no máximo
// partyLevelGap níveis abaixo do mais alto), exatamente como as missões e as conquistas de
// abate. Cada um conta 1 abate inteiro (não uma fração): o bestiário mede o que o jogador
// enfrentou, e dividir faria a caça em grupo valer menos que a sozinho. No raid boss, conta
// para todo mundo que causou dano e ainda está na zona (como a conquista do raid), não só
// para quem passou dos 5% de XP: o Ozrhal precisa de 1 abate e todos estavam na luta.
//
// Gravação: junto com o personagem (characters.bestiary, migração 00031), no salvamento
// periódico e na saída; um marco ou uma família completa pede o salvamento logo (saveSoon),
// como as conquistas.

type bestiary struct {
	kills    map[string]game.BestiaryKill
	families map[string]int64 // família completa → quando (Unix, ms)
}

// loadBestiary monta o estado salvo. Abates de monstros que não existem nesta zona ficam
// guardados (outra zona ou um monstro que volta); famílias que deixaram de existir saem.
func loadBestiary(st game.BestiaryState) bestiary {
	b := bestiary{kills: map[string]game.BestiaryKill{}, families: map[string]int64{}}
	for id, k := range st.Kills {
		if k.N <= 0 || id == "" {
			continue
		}
		k.N = min(k.N, game.BestiaryMaxKills)
		k.Lo, k.Hi = max(0, k.Lo), max(0, k.Hi)
		b.kills[id] = k
	}
	for id, at := range st.Families {
		if game.BestiaryFamilyByID(id) != nil {
			b.families[id] = at
		}
	}
	return b
}

func (b *bestiary) state() game.BestiaryState {
	st := game.BestiaryState{}
	if len(b.kills) > 0 {
		st.Kills = maps.Clone(b.kills)
	}
	if len(b.families) > 0 {
		st.Families = maps.Clone(b.families)
	}
	return st
}

// view: o estado para as contas de game (sem copiar; só leitura).
func (b *bestiary) view() game.BestiaryState {
	return game.BestiaryState{Kills: b.kills, Families: b.families}
}

// bestiaryBonus: o bônus de dano do bestiário de src contra dst (0 se não for um jogador
// batendo num monstro). Entra no multiplicador do golpe (hitInput).
func (z *Zone) bestiaryBonus(src, dst *entity) float64 {
	if src.pl == nil || dst.mon == nil {
		return 0
	}
	return game.BestiaryBonus(dst.mon.def, src.pl.bst.kills[dst.mon.def.ID].N)
}

// bestiaryKill conta o abate de dead para p (ver as regras de crédito no topo do arquivo).
func (z *Zone) bestiaryKill(p, dead *entity) {
	if p.pl == nil || dead.mon == nil {
		return
	}
	def := dead.mon.def
	e := game.BestiaryEntryByID(def.ID)
	if e == nil {
		return
	}
	b := &p.pl.bst
	k := b.kills[def.ID]
	before := game.BestiaryMark(def, k.N)
	famBefore := k.N >= game.BestiaryFamilyNeed(def)
	if k.N < game.BestiaryMaxKills {
		k.N++
	}
	if lvl := max(1, dead.level); k.Lo == 0 || lvl < k.Lo {
		k.Lo = lvl
	}
	k.Hi = max(k.Hi, dead.level)
	b.kills[def.ID] = k
	after := game.BestiaryMark(def, k.N)

	upd := &pb.Bestiary{Entries: []*pb.BestiaryEntry{bestiaryEntryPB(b, e)}, DamageBonus: game.BestiaryDamageBonus}
	var notes []*pb.BestiaryMark
	for m := before + 1; m <= after; m++ {
		if game.BestiaryOneKill(def) && m < 3 {
			continue // chefe e raro: um aviso só (a entrada inteira)
		}
		notes = append(notes, &pb.BestiaryMark{Id: def.ID, Name: def.Name, Mark: uint32(m), Kills: uint32(k.N), Text: bestiaryMarkText(def, m)})
	}
	if f := e.Family; f != nil && !famBefore && k.N >= game.BestiaryFamilyNeed(def) {
		if _, done := b.families[f.ID]; !done {
			if d, t := b.view().FamilyProgress(f); d >= t {
				b.families[f.ID] = z.cfg.Clock().UnixMilli()
				notes = append(notes, &pb.BestiaryMark{Family: f.ID, Name: f.Name,
					Text: fmt.Sprintf("Família %s completa! Título «%s» e a cor de roupa «%s» no barbeiro.", f.Name, f.Title, f.Outfit.Name)})
			}
		}
		upd.Families = append(upd.Families, bestiaryFamilyPB(b, f))
	}
	z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_Bestiary{Bestiary: upd}})
	if len(notes) == 0 {
		return
	}
	z.saveSoon(p)
	for _, n := range notes {
		if n.Family != "" {
			z.log.Info("bestiary family", "id", p.id, "name", p.name, "family", n.Family)
			z.track(p, "bestiary", "family:"+n.Family, 0, nil)
		} else {
			z.track(p, "bestiary", n.Id, int64(n.Mark), nil)
		}
	}
	// O aviso sai depois do snapshot: o abate aparece antes do marco.
	z.afterFlush(func() {
		if z.players[p.id] != p {
			return
		}
		for _, n := range notes {
			if n.Family != "" {
				z.system(p, "Bestiário: "+n.Text)
			} else {
				z.system(p, fmt.Sprintf("Bestiário: %s — %d abate(s): %s", n.Name, n.Kills, n.Text))
			}
			z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_BestiaryMark{BestiaryMark: n}})
		}
	})
}

// bestiaryMarkText: o que o marco m (1–3) abriu.
func bestiaryMarkText(def *game.MonsterDef, m int) string {
	if game.BestiaryOneKill(def) {
		return "entrada completa no Bestiário."
	}
	switch m {
	case 1:
		return "nível e vida no Bestiário."
	case 2:
		return "o que ele deixa cair no Bestiário."
	}
	return fmt.Sprintf("+%.0f%% de dano contra %s.", game.BestiaryDamageBonus*100, def.Name)
}

// bestiaryEntryPB: a entrada como o dono vê (os campos de cada marco só depois do marco).
func bestiaryEntryPB(b *bestiary, e *game.BestiaryEntryDef) *pb.BestiaryEntry {
	def := e.Monster
	k := b.kills[def.ID]
	marks := game.BestiaryMarks(def)
	mark := game.BestiaryMark(def, k.N)
	out := &pb.BestiaryEntry{
		Id: def.ID, Model: def.Model, Kills: uint32(k.N), Mark: uint32(mark),
		Marks: []uint32{uint32(marks[0]), uint32(marks[1]), uint32(marks[2])},
		Boss:  game.BestiaryOneKill(def), FamilyNeed: uint32(game.BestiaryFamilyNeed(def)),
	}
	if e.Family != nil {
		out.Family = e.Family.ID
	}
	if k.N == 0 {
		return out // silhueta
	}
	out.Name = def.Name
	lo, hi := max(1, k.Lo), max(1, k.Lo, k.Hi)
	if mark >= 1 {
		hpLo, gMin, _ := game.BestiaryLevelStats(def, lo)
		hpHi, _, gMax := game.BestiaryLevelStats(def, hi)
		out.LevelMin, out.LevelMax, out.HpMin, out.HpMax = uint32(lo), uint32(hi), uint32(hpLo), uint32(hpHi)
		if mark >= 2 {
			out.GoldChance, out.GoldMin, out.GoldMax, out.Gear = float32(def.GoldChance), uint32(gMin), uint32(gMax), !def.NoGear
			for _, l := range def.Loot {
				name := l.ItemID
				if it := game.Items[l.ItemID]; it != nil {
					name = it.Name
				}
				out.Drops = append(out.Drops, &pb.BestiaryDrop{ItemId: l.ItemID, Name: name, Chance: float32(l.Chance), Min: uint32(l.Min), Max: uint32(l.Max)})
			}
		}
	}
	out.DamageBonus = float32(game.BestiaryBonus(def, k.N))
	return out
}

func bestiaryFamilyPB(b *bestiary, f *game.BestiaryFamilyDef) *pb.BestiaryFamily {
	done, total := b.view().FamilyProgress(f)
	out := &pb.BestiaryFamily{
		Id: f.ID, Name: f.Name, Done: uint32(done), Total: uint32(total),
		TitleId: f.TitleID(), Title: f.Title, Outfit: uint32(f.OutfitIndex()), OutfitName: f.Outfit.Name,
	}
	if at, ok := b.families[f.ID]; ok {
		out.Complete, out.CompletedAtMs = true, at
	}
	return out
}

// sendBestiary manda o bestiário inteiro.
func (z *Zone) sendBestiary(p *entity) {
	b := &p.pl.bst
	cat := game.BestiaryCatalog()
	msg := &pb.Bestiary{Full: true, Entries: make([]*pb.BestiaryEntry, 0, len(cat)), DamageBonus: game.BestiaryDamageBonus}
	for _, e := range cat {
		msg.Entries = append(msg.Entries, bestiaryEntryPB(b, e))
	}
	for _, f := range game.BestiaryFamilies {
		msg.Families = append(msg.Families, bestiaryFamilyPB(b, f))
	}
	z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_Bestiary{Bestiary: msg}})
}

// bestiaryOnJoin: completa as famílias que já estavam completas sem aviso (o catálogo
// mudou), tira um título de família que o personagem não tem e manda o bestiário.
func (z *Zone) bestiaryOnJoin(p *entity) {
	b := &p.pl.bst
	for _, f := range game.BestiaryFamilies {
		if _, done := b.families[f.ID]; done {
			continue
		}
		if d, t := b.view().FamilyProgress(f); t > 0 && d >= t {
			b.families[f.ID] = z.cfg.Clock().UnixMilli()
			z.saveSoon(p)
		}
	}
	if f := game.BestiaryFamilyByTitle(p.pl.ach.title); f != nil {
		if _, ok := b.families[f.ID]; !ok {
			p.pl.ach.title = ""
		}
	}
	z.sendBestiary(p)
}

// bestiaryHasTitle: o título id é de uma família que o personagem completou.
func (z *Zone) bestiaryHasTitle(p *entity, id string) bool {
	f := game.BestiaryFamilyByTitle(id)
	if f == nil {
		return false
	}
	_, ok := p.pl.bst.families[f.ID]
	return ok
}

// bestiaryOutfitLocked: a cor de roupa é do bestiário e o personagem não completou a
// família (o texto do aviso; "" = pode usar).
func (z *Zone) bestiaryOutfitLocked(p *entity, outfit int) string {
	f := game.BestiaryFamilyByOutfit(outfit)
	if f == nil {
		return ""
	}
	if _, ok := p.pl.bst.families[f.ID]; ok {
		return ""
	}
	return fmt.Sprintf("A cor «%s» é do Bestiário: complete a família %s para usá-la.", f.Outfit.Name, f.Name)
}

// onBestiaryCmd: a janela abriu e pediu o bestiário.
func (z *Zone) onBestiaryCmd(p *entity, m *pb.BestiaryCmd) {
	if m.Kind != pb.BestiaryCmdKind_BESTIARY_CMD_GET {
		return
	}
	z.sendBestiary(p)
}
