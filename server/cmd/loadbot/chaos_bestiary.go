package main

import (
	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// actBestiary: abrir o Bestiário (pedir o estado), às vezes com um tipo que não existe ou
// tentando usar o título de uma família que o bot não completou (a zona recusa).
func actBestiary(b *chaosBot, s *chaosSession, r [6]uint64) {
	fam := game.BestiaryFamilies[pickN(r[2], len(game.BestiaryFamilies))]
	switch p := pct(r[1]); {
	case p < 70:
		b.emit(s, "bestiario:get", &pb.ClientMessage{Msg: &pb.ClientMessage_BestiaryCmd{BestiaryCmd: &pb.BestiaryCmd{Kind: pb.BestiaryCmdKind_BESTIARY_CMD_GET}}})
	case p < 85:
		b.emit(s, "bestiario:titulo", &pb.ClientMessage{Msg: &pb.ClientMessage_AchievementCmd{AchievementCmd: &pb.AchievementCmd{
			Kind: pb.AchievementCmdKind_ACHIEVEMENT_CMD_SET_TITLE, Id: fam.TitleID()}}})
	default:
		b.emit(s, "bestiario!lixo", &pb.ClientMessage{Msg: &pb.ClientMessage_BestiaryCmd{BestiaryCmd: &pb.BestiaryCmd{Kind: pb.BestiaryCmdKind(pickN(r[3], 6))}}})
	}
}
