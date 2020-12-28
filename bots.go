package main

import (
 "log"
 "time"
 )

type triggerRequest struct{
    t time.Time
    game string
    botName string
}

var botTriggerQueue chan triggerRequest

func processQueue() {
    tr := <-botTriggerQueue
    lobby.mu.Lock()
    defer lobby.mu.Unlock()

    g, ok := lobby.games[tr.game]
    if !ok || g.status != GAME_STATUS_RUNNING {
        log.Printf("Game %s no longer exists, bot %s dies", tr.game, tr.botName)
        return
    }
    triggerBot(g, tr.botName)
}

func triggerBot(g *Game, botname string) {
    // Get state and process it
    // Make a decision and necessary callS
}
