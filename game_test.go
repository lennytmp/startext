package main

import (
	"sync"
	"testing"
	"time"
)

const (
	TESTGAME = "test"
)

func basicLobbyGame() *Lobby {
	lobby := newLobby()
	g := newGame()
	g.lastSim = time.Now().Add(-3500 * time.Millisecond)
	lobby.games[TESTGAME] = g
	g.status = GAME_STATUS_RUNNING
	g.Players["0"] = &Player{50, "", sync.Mutex{}}
	g.Players["1"] = &Player{50, "", sync.Mutex{}}

	g.Objects = append(g.Objects, CommandCenter("0", 0))
	g.Objects = append(g.Objects, CommandCenter("1", 1))
	return lobby
}

func TestAttackDecreasesHp(t *testing.T) {
	l := basicLobbyGame()
	g := l.games[TESTGAME]
	g.Objects = append(g.Objects, SCV("0", 1))
	updLobby(l)
	cmd1 := g.Objects[1]
	if cmd1.Hp >= cmd1.HpMax {
		t.Errorf("Command center was supposed to be damaged but has %d hp out of %d", cmd1.Hp, cmd1.HpMax)
	}
}

func TestGameEnds(t *testing.T) {
	l := basicLobbyGame()
	g := l.games[TESTGAME]
	g.Objects = append(g.Objects, SCV("0", 1))
	g.Objects[1].Hp = 1
	updLobby(l)
	if g.status != GAME_STATUS_FINISHED {
		t.Errorf("Expected the game to finish but it has status %s", g.status)
	}
	if g.Players["0"].Outcome != VICTORY {
		t.Errorf("Expected player0 to be victorious, but got %q outcome", g.Players["0"].Outcome)
	}
	if g.Players["1"].Outcome != ELIMINATED {
		t.Errorf("Expected player0 to be eliminated, but got %q outcome", g.Players["1"].Outcome)
	}
}
