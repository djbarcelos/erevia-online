package zone

import (
	"math"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"

	"erevia/server/internal/pb"
)

// Área de interesse (AOI): cada jogador só recebe jogadores e monstros a até aoiRadius
// dele (mais os colegas de grupo, em qualquer lugar). Quem entra no raio recebe um
// Spawn; quem sai de aoiRadius+aoiSlack recebe um Despawn (a folga evita piscar na
// borda). O snapshot é montado por jogador com o que ele conhece. Sem isso, todo mundo
// recebia todo mundo: a banda crescia com o quadrado dos jogadores (teste de carga, #83).
//
// NPCs, objetos (estandartes), itens no chão, chat e anúncios continuam para todos.

const (
	aoiEvery  = 4 // ticks entre atualizações da AOI (5×/s a 20 ticks/s)
	aoiRadius = 70.0
	aoiSlack  = 10.0
	aoiCell   = 20.0
)

type cellKey struct{ x, z int32 }

func cellOf(p Vec2) cellKey {
	return cellKey{int32(math.Floor(p.X / aoiCell)), int32(math.Floor(p.Z / aoiCell))}
}

// aoiVisible: entra na AOI (jogadores e monstros cujo corpo ainda está no mundo).
func aoiVisible(e *entity) bool {
	return e.mon == nil || !e.mon.hidden
}

// aoiGrid indexa jogadores e monstros visíveis por célula.
func (z *Zone) aoiGrid() map[cellKey][]*entity {
	g := make(map[cellKey][]*entity, len(z.players)+len(z.monsters)/4)
	for _, p := range z.players {
		c := cellOf(p.pos)
		g[c] = append(g[c], p)
	}
	for _, m := range z.monsters {
		if aoiVisible(m) {
			c := cellOf(m.pos)
			g[c] = append(g[c], m)
		}
	}
	return g
}

// aoiAround chama f para cada entidade da grade a até r de p.
func aoiAround(g map[cellKey][]*entity, p Vec2, r float64, f func(*entity)) {
	lo, hi := cellOf(Vec2{X: p.X - r, Z: p.Z - r}), cellOf(Vec2{X: p.X + r, Z: p.Z + r})
	for cx := lo.x; cx <= hi.x; cx++ {
		for cz := lo.z; cz <= hi.z; cz++ {
			for _, e := range g[cellKey{cx, cz}] {
				if dist(e.pos, p) <= r {
					f(e)
				}
			}
		}
	}
}

// aoiSees: p deve conhecer e (r com ou sem a folga).
func (z *Zone) aoiSees(p, e *entity, r float64) bool {
	if e == p {
		return true
	}
	if e.pl != nil && p.pl.party != nil && e.pl.party == p.pl.party {
		return true
	}
	return dist(p.pos, e.pos) <= r
}

// knownEntity: o que o cliente recebeu no Spawn (a entidade e o spawnSeq daquele Spawn).
type knownEntity struct {
	e   *entity
	seq uint32
}

// updateAOI manda Spawn/Despawn conforme as entidades entram e saem do raio de cada um.
// Roda a cada aoiEvery ticks: quem entra no raio aparece no máximo ~200 ms depois.
func (z *Zone) updateAOI() {
	g := z.aoiGrid()
	for _, p := range z.players {
		known := p.pl.known
		if known == nil {
			known = map[uint32]knownEntity{}
			p.pl.known = known
		}
		// Quem saiu (do mundo, do raio com folga ou do grupo) ou renasceu (Spawn novo abaixo).
		for id, k := range known {
			e := k.e
			if e.gone || !aoiVisible(e) || e.spawnSeq != k.seq || !z.aoiSees(p, e, aoiRadius+aoiSlack) {
				delete(known, id)
				z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_Despawn{Despawn: &pb.Despawn{Id: id}}})
			}
		}
		// Quem entrou.
		add := func(e *entity) {
			if _, ok := known[e.id]; !ok {
				known[e.id] = knownEntity{e: e, seq: e.spawnSeq}
				z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_Spawn{Spawn: &pb.Spawn{Entity: z.entityPB(e)}}})
			}
		}
		known[p.id] = knownEntity{e: p, seq: p.spawnSeq} // o próprio jogador já veio no Welcome
		aoiAround(g, p.pos, aoiRadius, add)
		if p.pl.party != nil {
			for _, m := range z.partyMembers(p.pl.party) {
				add(m)
			}
		}
	}
}

// welcomeEntities: o que o jogador novo já vê (e passa a conhecer).
func (z *Zone) welcomeEntities(p *entity) []*pb.Entity {
	p.pl.known = map[uint32]knownEntity{}
	var out []*pb.Entity
	add := func(e *entity) {
		if _, ok := p.pl.known[e.id]; !ok {
			p.pl.known[e.id] = knownEntity{e: e, seq: e.spawnSeq}
			out = append(out, z.entityPB(e))
		}
	}
	add(p)
	aoiAround(z.aoiGrid(), p.pos, aoiRadius, add)
	return out
}

// ---------- Snapshot por jogador ----------
//
// Cada estado e cada evento do tick é codificado UMA vez (campo protobuf já com a tag);
// o snapshot de cada jogador é só a colagem dos pedaços que ele conhece. Serializar um
// snapshot inteiro por jogador custava ~30% do tick com 400 jogadores.

// Campos de pb.Snapshot e de pb.ServerMessage (proto/erevia/v1/game.proto).
const (
	snapFieldTick       = 1
	snapFieldStates     = 2
	snapFieldHp         = 3
	snapFieldCombat     = 4
	snapFieldEffects    = 5
	snapFieldDeaths     = 6
	snapFieldRevives    = 7
	snapFieldLevelUps   = 8
	snapFieldEffectEnds = 9
	snapFieldCasts      = 10
	snapFieldTelegraphs = 11
	serverFieldSnapshot = 4
)

// encField codifica m como o campo num (mensagem aninhada).
func encField(num protowire.Number, m proto.Message) []byte {
	inner, _ := proto.Marshal(m)
	b := protowire.AppendTag(nil, num, protowire.BytesType)
	return protowire.AppendBytes(b, inner)
}

// tickEvent: um evento do tick já codificado e as entidades que o tornam relevante.
type tickEvent struct {
	a, b uint32 // relevante para quem conhece a ou b (0 = ninguém)
	enc  []byte
}

// encodeFrame codifica os eventos do tick uma vez.
func encodeFrame(all *pb.Snapshot) []tickEvent {
	var ev []tickEvent
	for _, c := range all.Combat {
		ev = append(ev, tickEvent{a: c.SourceId, b: c.TargetId, enc: encField(snapFieldCombat, c)})
	}
	for _, e := range all.Effects {
		ev = append(ev, tickEvent{a: e.TargetId, enc: encField(snapFieldEffects, e)})
	}
	for _, r := range all.Revives {
		ev = append(ev, tickEvent{a: r.Id, enc: encField(snapFieldRevives, r)})
	}
	for _, l := range all.LevelUps {
		ev = append(ev, tickEvent{a: l.Id, enc: encField(snapFieldLevelUps, l)})
	}
	for _, e := range all.EffectEnds {
		ev = append(ev, tickEvent{a: e.TargetId, enc: encField(snapFieldEffectEnds, e)})
	}
	for _, c := range all.Casts {
		ev = append(ev, tickEvent{a: c.SourceId, enc: encField(snapFieldCasts, c)})
	}
	// Avisos no chão: para quem conhece o monstro ou o marcado (nunca para o mapa todo).
	for _, t := range all.Telegraphs {
		ev = append(ev, tickEvent{a: t.SourceId, b: t.FollowId, enc: encField(snapFieldTelegraphs, t)})
	}
	return ev
}

// snapshotBytes monta o ServerMessage{Snapshot} de p (nil = nada para ele).
func snapshotBytes(p *entity, tick uint32, deaths []uint32, events []tickEvent, buf *[]byte) []byte {
	known := p.pl.known
	has := func(id uint32) bool {
		if id == 0 {
			return false
		}
		_, ok := known[id]
		return ok
	}
	body := (*buf)[:0]
	defer func() { *buf = body }()
	body = protowire.AppendTag(body, snapFieldTick, protowire.VarintType)
	body = protowire.AppendVarint(body, uint64(tick))
	head := len(body)
	for _, k := range known {
		e := k.e
		if e.encTick != tick || e.gone || e.spawnSeq != k.seq {
			continue
		}
		body = append(body, e.encState...)
		body = append(body, e.encHP...)
	}
	for _, ev := range events {
		if has(ev.a) || has(ev.b) {
			body = append(body, ev.enc...)
		}
	}
	// deaths: repeated uint32 empacotado (proto3).
	var packed []byte
	for _, id := range deaths {
		if has(id) {
			packed = protowire.AppendVarint(packed, uint64(id))
		}
	}
	if len(packed) > 0 {
		body = protowire.AppendTag(body, snapFieldDeaths, protowire.BytesType)
		body = protowire.AppendBytes(body, packed)
	}
	if len(body) == head {
		return nil
	}
	out := protowire.AppendTag(make([]byte, 0, len(body)+8), serverFieldSnapshot, protowire.BytesType)
	return protowire.AppendBytes(out, body)
}

func snapshotEmpty(s *pb.Snapshot) bool {
	return len(s.States)+len(s.Hp)+len(s.Combat)+len(s.Effects)+len(s.Deaths)+len(s.Revives)+
		len(s.LevelUps)+len(s.EffectEnds)+len(s.Casts)+len(s.Telegraphs) == 0
}

// sendSnapshots manda a cada jogador o seu pedaço do snapshot.
func (z *Zone) sendSnapshots(all *pb.Snapshot) {
	events := encodeFrame(all)
	var slow []uint32
	var buf []byte
	for id, p := range z.players {
		data := snapshotBytes(p, all.Tick, all.Deaths, events, &buf)
		if data == nil {
			continue
		}
		if !p.pl.peer.Send(data) {
			slow = append(slow, id)
		}
	}
	for _, id := range slow {
		z.dropSlow(id)
	}
}
