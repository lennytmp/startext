package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

type triggerRequest struct {
	t       time.Time
	game    string
	botName string
}

var (
	botTriggerQueue           chan triggerRequest
	makeBotRequestOverridable = makeBotRequest
)

func processBotQueue() {
	tr := <-botTriggerQueue
	if time.Now().After(tr.t) {
		triggerBot(tr.game, tr.botName)
	} else {
		botTriggerQueue <- tr
	}
}

func makeBotRequest(url string) ([]byte, error) {
	var res []byte
	resp, err := http.Get("http://localhost:8182" + url)
	if err != nil {
		return res, fmt.Errorf("sending request failed %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return res, fmt.Errorf("bad status code: %v", resp.Status)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return res, fmt.Errorf("couldn't read all response body %v", err)
	}
	return body, nil
}

func triggerBot(gameName string, botName string) {
	{ // Get state and process it
		rURL := fmt.Sprintf("/?player=%s", botName)
		resp, err := makeBotRequestOverridable(rURL)
		if err != nil {
			log.Printf("ERROR: Making request %s for bot %s game %s failed with %v", rURL, botName, gameName, err)
		}
		g := &Game{}
		err = json.Unmarshal(resp, g)
		if err != nil {
			log.Printf("ERROR: Bot %s got resp %s from request %s for game %s, but couldn't transform it to Game: %v", botName, resp, gameName, err)
		}
		minerals := g.Players[botName].Minerals
		locationId := 0
		var commandCenter GameObject
		for _, gob := range g.Objects {
			if gob.Owner == botName && gob.Type == GAME_BUILDING_COMMAND_CENTER {
				locationId = gob.Location
				commandCenter = gob
			}
		}
		if minerals >= 50 && commandCenter.Task == (Task{}) {
			rURL := fmt.Sprintf("/?player=%s&location_id=%d&build_scv", botName, locationId)
			_, err := makeBotRequestOverridable(rURL)
			if err != nil {
				log.Printf("ERROR: Making request %s for bot %s game %s failed with %v", rURL, botName, gameName, err)
			}
		}
	}
	botTriggerQueue <- triggerRequest{time.Now().Add(30 * time.Second), gameName, botName}
}
