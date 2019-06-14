package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	uuid "github.com/satori/go.uuid"
)

func makeEvent(srcID, metric string, min, max float64) []byte {

	event := struct {
		SourceID string  `json:"source_id"`
		EventID  string  `json:"event_id"`
		EventTs  int64   `json:"event_time"`
		Metric   string  `json:"metric"`
		Value    float64 `json:"value"`
	}{
		SourceID: srcID,
		EventID:  fmt.Sprintf("eid-%s", uuid.NewV4().String()),
		EventTs:  time.Now().UTC().Unix(),
		Metric:   metric,
		Value:    min + rand.Float64()*(max-min),
	}

	data, _ := json.Marshal(event)
	logger.Printf("%s[%s]: %s", metric, srcID, string(data))
	return data
}
