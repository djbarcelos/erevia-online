package zone

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strings"
	"time"
)

// Portal de administração com várias zonas: o Manager responde pelas mesmas interfaces
// que o *Zone (api.AdminZone, OpsZone, MetricsZone, PlayersZone, BugZone e o
// PanicReporter dos alertas) e manda cada pedido para quem é dono do estado:
//   - um personagem: a zona onde ele está (online ou atravessando um portão), a de onde
//     saiu há pouco (o recent dela vale mais que o banco) ou, offline de verdade, a zona
//     gravada no banco, que é para onde o próximo login vai (a trava e a hora da mudança
//     ficam lá; ver admin_actions.go);
//   - mudanças na conta (silêncio, ban, GM): todas as zonas;
//   - anúncios (agora e agendados), manutenção e XP em dobro: todas as zonas (cada uma
//     avisa os seus jogadores; o retrato da Operação é o do mapa principal);
//   - invasão e chefes: o mapa principal (é onde eles existem).

// ZoneLookup diz o mapa gravado no banco para um personagem ("" = não sabe).
type ZoneLookup func(ctx context.Context, id uint32) (string, error)

// SetZoneLookup liga a consulta do mapa no banco (antes de servir o portal).
func (m *Manager) SetZoneLookup(fn ZoneLookup) { m.lookup = fn }

// ownerOf: a zona dona do estado do personagem agora (ver o começo do arquivo).
func (m *Manager) ownerOf(ctx context.Context, id uint32) *Zone {
	if z := m.route(ctx, id); z != nil {
		return z
	}
	if m.lookup != nil {
		if mapID, err := m.lookup(ctx, id); err == nil {
			if z := m.Zone(mapID); z != nil {
				return z
			}
		}
	}
	return m.Home()
}

// AdminOnline: quem está online em todas as zonas, por nome.
func (m *Manager) AdminOnline(ctx context.Context) ([]AdminOnlinePlayer, error) {
	var out []AdminOnlinePlayer
	var errs []error
	for _, z := range m.zones {
		list, err := z.AdminOnline(ctx)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if z != m.Home() {
			for i := range list {
				list[i].Region = z.mapTitle()
			}
		}
		out = append(out, list...)
	}
	slices.SortFunc(out, func(a, b AdminOnlinePlayer) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	return out, errors.Join(errs...)
}

func (m *Manager) AdminCharacter(ctx context.Context, id uint32, accountID int64) (AdminLive, error) {
	return m.ownerOf(ctx, id).AdminCharacter(ctx, id, accountID)
}

// AdminEdit roda na zona dona. Offline, a zona trava o personagem; se nesse meio-tempo
// ele entrou em outra zona (portão ou login), a trava é solta e o pedido vai para lá.
func (m *Manager) AdminEdit(ctx context.Context, id uint32, e AdminEdit) (AdminActionResult, error) {
	for range 3 {
		z := m.ownerOf(ctx, id)
		r, err := z.AdminEdit(ctx, id, e)
		if err != nil || r.Where != WhereOffline || m.ownerOf(ctx, id) == z {
			return r, err
		}
		if err := z.AdminRelease(ctx, id, r.Hold); err != nil {
			return AdminActionResult{}, err
		}
	}
	return AdminActionResult{}, ErrCharacterBusy
}

func (m *Manager) AdminRelease(ctx context.Context, id uint32, hold uint64) error {
	return m.ownerOf(ctx, id).AdminRelease(ctx, id, hold)
}

func (m *Manager) AdminKick(ctx context.Context, id uint32, reason string) (string, error) {
	z := m.bus.zoneOf(id)
	if z == nil {
		return "", ErrNotOnline
	}
	return z.AdminKick(ctx, id, reason)
}

// AdminAccountChanged: a conta pode ter personagens em qualquer zona.
func (m *Manager) AdminAccountChanged(ctx context.Context, c AccountChange) ([]string, error) {
	var out []string
	var errs []error
	for _, z := range m.zones {
		names, err := z.AdminAccountChanged(ctx, c)
		out = append(out, names...)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return out, errors.Join(errs...)
}

// PrepareBug: o /bug pelo HTTP vale na zona onde o personagem está.
func (m *Manager) PrepareBug(ctx context.Context, accountID, characterID int64, text string) (BugReport, error) {
	z := m.Home()
	if characterID > 0 && characterID <= int64(^uint32(0)) {
		if oz := m.bus.zoneOf(uint32(characterID)); oz != nil {
			z = oz
		}
	}
	return z.PrepareBug(ctx, accountID, characterID, text)
}

// LiveStats soma as zonas (as regiões dos outros mapas levam o nome do mapa); o tick é o
// do mapa principal.
func (m *Manager) LiveStats(ctx context.Context) (LiveStats, error) {
	out, err := m.Home().LiveStats(ctx)
	if err != nil {
		return out, err
	}
	out.ByRegion = maps.Clone(out.ByRegion)
	for _, z := range m.zones[1:] {
		s, err := z.LiveStats(ctx)
		if err != nil {
			return out, err
		}
		out.Online += s.Online
		out.Monsters += s.Monsters
		out.Dead += s.Dead
		out.Ground += s.Ground
		for _, n := range s.ByRegion {
			out.ByRegion[z.mapTitle()] += n
		}
	}
	return out, nil
}

// ---------- Operação: o mapa principal ----------

func (m *Manager) AdminOps(ctx context.Context) (OpsStatus, error) {
	s, err := m.Home().AdminOps(ctx)
	s.Online = m.bus.count()
	return s, err
}

func (m *Manager) AdminAnnounce(ctx context.Context, title, text string) (int, error) {
	total := 0
	for _, z := range m.zones {
		n, err := z.AdminAnnounce(ctx, title, text)
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

func (m *Manager) AdminSchedule(ctx context.Context, a ScheduledAnnouncement) error {
	for _, z := range m.zones {
		if err := z.AdminSchedule(ctx, a); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) AdminUnschedule(ctx context.Context, id int64) (bool, error) {
	found := false
	for _, z := range m.zones {
		ok, err := z.AdminUnschedule(ctx, id)
		if err != nil {
			return found, err
		}
		found = found || ok
	}
	return found, nil
}

// LoadScheduled: os anúncios agendados que sobreviveram ao reinício, em todas as zonas.
func (m *Manager) LoadScheduled(list []ScheduledAnnouncement) {
	for _, z := range m.zones {
		z.LoadScheduled(slices.Clone(list))
	}
}

func (m *Manager) AdminMaintenance(ctx context.Context, minutes float64, message, by string) (time.Time, error) {
	var at time.Time
	for i, z := range m.zones {
		t, err := z.AdminMaintenance(ctx, minutes, message, by)
		if err != nil {
			return at, err
		}
		if i == 0 {
			at = t
		}
	}
	return at, nil
}

func (m *Manager) AdminMaintenanceCancel(ctx context.Context) (bool, error) {
	found := false
	for _, z := range m.zones {
		ok, err := z.AdminMaintenanceCancel(ctx)
		if err != nil {
			return found, err
		}
		found = found || ok
	}
	return found, nil
}

func (m *Manager) AdminInvasion(ctx context.Context, action string) (string, error) {
	return m.Home().AdminInvasion(ctx, action)
}

// AdminXP2 liga (ou desliga) a XP em dobro em todas as zonas (cada uma avisa os seus); a
// resposta é a do mapa principal.
func (m *Manager) AdminXP2(ctx context.Context, hours float64) (string, error) {
	out, err := m.Home().AdminXP2(ctx, hours)
	if err != nil {
		return out, err
	}
	for _, z := range m.zones[1:] {
		if _, err := z.AdminXP2(ctx, hours); err != nil {
			return out, err
		}
	}
	return out, nil
}

func (m *Manager) AdminBossRespawn(ctx context.Context, id uint32) (string, error) {
	return m.Home().AdminBossRespawn(ctx, id)
}
