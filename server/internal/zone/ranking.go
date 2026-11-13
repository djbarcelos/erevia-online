package zone

import (
	"context"
	"time"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// Ranking: um worker calcula no banco a cada rankingEvery (top 50 de nível, de vitórias
// PvP e de clãs, mais a posição de cada personagem e clã) e entrega o resultado à zona
// por evento. A zona só guarda em memória e responde aos pedidos: a goroutine da zona
// nunca consulta o banco.

const (
	rankingEvery = 5 * time.Minute
	rankingLimit = 50
	rankingGap   = 2.0 // segundos entre pedidos de um mesmo jogador
)

// RankRow é uma linha do ranking de personagens.
type RankRow struct {
	CharacterID int64
	Name        string
	Class       string
	Level       int
	PvPKills    int
	ClanName    string
	TitleID     string // conquista do título em uso
}

// ClanRankRow é uma linha do ranking de clãs.
type ClanRankRow struct {
	ClanID   int64
	Name     string
	Members  int
	LevelSum int
	LevelAvg float64
	Leader   string
}

// RankingData é o resultado de um cálculo: as listas (já ordenadas) e as posições de
// todos (personagem ou clã → posição, começando em 1).
type RankingData struct {
	At       time.Time
	Level    []RankRow
	PvP      []RankRow
	Clans    []ClanRankRow
	LevelPos map[int64]int
	PvPPos   map[int64]int
	ClanPos  map[int64]int
}

// RankingBackend calcula o ranking no banco (implementado com o store).
type RankingBackend interface {
	Rankings(ctx context.Context, limit int) (RankingData, error)
}

// SetRankingBackend liga o ranking ao banco (antes de Run). Sem ele, o ranking fica vazio.
func (z *Zone) SetRankingBackend(b RankingBackend) { z.rankingBackend = b }

// rankingCache: as listas já em protobuf (as mesmas para todo pedido) e as posições.
type rankingCache struct {
	data  RankingData
	level []*pb.RankEntry
	pvp   []*pb.RankEntry
	clans []*pb.ClanRankEntry
}

type rankingEvent struct{ data RankingData }

func (rankingEvent) isEvent() {}

func (z *Zone) runRankingWorker(ctx context.Context) {
	if z.rankingBackend == nil {
		return
	}
	tick := time.NewTicker(rankingEvery)
	defer tick.Stop()
	for {
		cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		data, err := z.rankingBackend.Rankings(cctx, rankingLimit)
		cancel()
		if err != nil {
			z.log.Warn("ranking", "err", err) // fica o cálculo anterior
			z.ReportDBError("ranking", err)
		} else {
			z.post(ctx, rankingEvent{data: data})
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

// setRanking guarda um cálculo novo (na goroutine da zona).
func (z *Zone) setRanking(d RankingData) {
	c := &rankingCache{data: d}
	rows := func(list []RankRow) []*pb.RankEntry {
		out := make([]*pb.RankEntry, len(list))
		for i, r := range list {
			out[i] = &pb.RankEntry{
				Pos: uint32(i + 1), CharacterId: uint64(r.CharacterID), Name: r.Name, Class: r.Class,
				Level: uint32(r.Level), PvpKills: uint32(r.PvPKills), ClanName: r.ClanName, Title: game.TitleOf(r.TitleID),
			}
		}
		return out
	}
	c.level, c.pvp = rows(d.Level), rows(d.PvP)
	c.clans = make([]*pb.ClanRankEntry, len(d.Clans))
	for i, r := range d.Clans {
		c.clans[i] = &pb.ClanRankEntry{
			Pos: uint32(i + 1), Id: uint64(r.ClanID), Name: r.Name, Members: uint32(r.Members),
			LevelSum: uint32(r.LevelSum), LevelAvg: float32(r.LevelAvg), Leader: r.Leader,
		}
	}
	z.ranking = c
}

// sendRanking responde ao pedido da janela (com a posição do próprio personagem).
func (z *Zone) sendRanking(p *entity) {
	if z.now < p.pl.rankingNext {
		return
	}
	p.pl.rankingNext = z.now + rankingGap
	out := &pb.Ranking{RefreshMs: uint32(rankingEvery.Milliseconds())}
	if c := z.ranking; c != nil {
		out.Level, out.Pvp, out.Clans = c.level, c.pvp, c.clans
		out.UpdatedAtMs = c.data.At.UnixMilli()
		out.MyLevelPos = uint32(c.data.LevelPos[int64(p.id)])
		out.MyPvpPos = uint32(c.data.PvPPos[int64(p.id)])
		if p.pl.clan != nil {
			out.MyClanPos = uint32(c.data.ClanPos[p.pl.clan.id])
		}
	}
	z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_Ranking{Ranking: out}})
}
