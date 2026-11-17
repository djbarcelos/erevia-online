package zone

import (
	"sort"
	"time"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// skillInfoPB descreve a skill para o cliente: valores do rank 1 nos campos principais
// (barra, dica e efeitos visuais), a tabela de ranks e o lugar na árvore.
func skillInfoPB(s *game.SkillDef) *pb.SkillInfo {
	info := &pb.SkillInfo{
		Id: s.ID, Name: s.Name, MpCost: uint32(s.MPCost(1)),
		CooldownMs: uint32(s.Cooldown(1) * 1000), Range: float32(s.Range), Target: pb.SkillTarget(s.Target),
		Description: s.Description, Fx: s.FX, Magic: s.Magic, Power: float32(s.Mult.At(1)),
		ExtraCrit: float32(s.ExtraCrit),
		Branch:    uint32(s.Branch), Tier: uint32(s.Tier), MaxRank: uint32(s.MaxRank), Passive: s.IsPassive(),
		LevelReq: uint32(s.LevelReq()), Splash: float32(s.Splash), Spec: s.Spec,
	}
	if s.IsPassive() {
		info.Target = pb.SkillTarget_SKILL_TARGET_UNSPECIFIED
	}
	if p := s.Parent(); p != nil {
		info.ParentId = p.ID
	}
	// Tecla padrão (só as duas iniciais; a barra de atalhos é do jogador).
	for i, st := range game.ClassSkills(s.Class) {
		if st.ID == s.ID {
			info.Key = uint32(i + 1)
		}
	}
	ef, self := s.OnTarget, false
	if ef == nil {
		ef, self = s.OnSelf, true
	}
	if ef != nil {
		info.Effect = pb.Effect(ef.Kind)
		info.EffectMs = uint32(ef.Duration.At(1) * 1000)
		info.EffectValue = float32(ef.Value.At(1))
		info.EffectOnSelf = self
	}
	for r := 1; r <= s.MaxRank; r++ {
		ri := &pb.SkillRankInfo{
			MpCost: uint32(s.MPCost(r)), CooldownMs: uint32(s.Cooldown(r) * 1000), Power: float32(s.Mult.At(r)),
			Text: s.Text(r),
		}
		if ef != nil {
			ri.EffectMs = uint32(ef.Duration.At(r) * 1000)
			ri.EffectValue = float32(ef.Value.At(r))
		}
		info.Ranks = append(info.Ranks, ri)
	}
	return info
}

// skillCatalog lista todas as skills (o cliente desenha as skills dos outros jogadores).
func skillCatalog() []*pb.SkillInfo {
	ids := make([]uint32, 0, len(game.Skills))
	for id := range game.Skills {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	out := make([]*pb.SkillInfo, 0, len(ids))
	for _, id := range ids {
		out = append(out, skillInfoPB(game.Skills[id]))
	}
	return out
}

// cancelQueuedSkill avisa o cliente que a skill agendada não foi usada (recarga intacta),
// para ele parar de mostrar a skill "na fila".
func (z *Zone) cancelQueuedSkill(e *entity) {
	if e.pl == nil || e.act.kind != actSkill || e.act.skill == nil {
		return
	}
	z.sendCooldown(e, e.act.skill.ID, e.pl.cooldowns[e.act.skill.ID]-z.now)
}

// saveCooldowns guarda as recargas em andamento de quem está saindo.
func (z *Zone) saveCooldowns(p *entity) {
	wall := z.cfg.Clock()
	saved := map[uint32]time.Time{}
	for id, ready := range p.pl.cooldowns {
		if left := ready - z.now; left > 0 {
			saved[id] = wall.Add(time.Duration(left * float64(time.Second)))
		}
	}
	if len(saved) > 0 {
		z.savedCooldowns[p.id] = saved
	} else {
		delete(z.savedCooldowns, p.id)
	}
}

// restoreCooldowns devolve as recargas que ainda não venceram a quem voltou.
func (z *Zone) restoreCooldowns(p *entity) {
	saved, ok := z.savedCooldowns[p.id]
	if !ok {
		return
	}
	delete(z.savedCooldowns, p.id)
	wall := z.cfg.Clock()
	for id, at := range saved {
		if left := at.Sub(wall).Seconds(); left > 0 {
			p.pl.cooldowns[id] = z.now + left
		}
	}
}
