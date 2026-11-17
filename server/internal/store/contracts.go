package store

import (
	"encoding/json"
	"fmt"

	"erevia/server/internal/game"
)

// Contratos e reputação (tunnel#157): duas colunas JSONB em characters (migração 00032),
// gravadas junto com o resto do personagem (mesma linha, mesma transação das moedas e dos
// itens da recompensa).

func contractsJSON(c game.ContractState, r game.ReputationState) ([]byte, []byte, error) {
	contracts, err := json.Marshal(c)
	if err != nil {
		return nil, nil, err
	}
	reputation, err := json.Marshal(r)
	if err != nil {
		return nil, nil, err
	}
	return contracts, reputation, nil
}

func parseContracts(contracts, reputation []byte, c *game.ContractState, r *game.ReputationState) error {
	if len(contracts) > 0 {
		if err := json.Unmarshal(contracts, c); err != nil {
			return fmt.Errorf("contracts: %w", err)
		}
	}
	if len(reputation) > 0 {
		if err := json.Unmarshal(reputation, r); err != nil {
			return fmt.Errorf("reputation: %w", err)
		}
	}
	return nil
}
