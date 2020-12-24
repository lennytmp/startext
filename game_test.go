package main

import (
	"sync"
	"testing"
)

func TestAttackDecreasesHp(t *testing.T) {
	lobby := newLobby()
	g := newGame()
	lobby.running["test"] = g
	g.status = GAME_STATUS_RUNNING
	g.Players["0"] = &Player{50, "", sync.Mutex{}}
	g.Players["1"] = &Player{50, "", sync.Mutex{}}

	g.Objects = append(g.Objects, CommandCenter("0", 0))
	g.Objects = append(g.Objects, CommandCenter("1", 1))
	g.Objects = append(g.Objects, SCV("0", 1))
	gameSim(g)
	cmd1 := g.Objects[1]
	if cmd1.Hp >= cmd1.HpMax {
		t.Errorf("Command center was supposed to be damaged but has %d hp out of %d", cmd1.Hp, cmd1.HpMax)
	}
}
