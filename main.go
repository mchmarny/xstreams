package main

import (
	"context"
	"flag"
	"log"
	"os"
	"time"
)

var (
	logger    = log.New(os.Stdout, "[EM] ", 0)
	projectID = flag.String("project", os.Getenv("GCP_PROJECT"), "GCP Project ID")
	deviceID  = flag.String("device", "device1", "ID of this deivce [device1]")
	tempTopic = flag.String("temp-topic", "eventmakertemp", "Temp topic name [eventmakertemp]")
	vibeTopic = flag.String("vibe-topic", "eventmakervibe", "Vibration topic name [eventmakervibe]")
	eventFreq = flag.String("freq", "5s", "Event frequency [5s]")
)

func main() {

	flag.Parse()

	ctx := context.Background()

	if *projectID == "" {
		projectID = getProjectID()
	}

	freq, err := time.ParseDuration(*eventFreq)
	failOnErr(err)

	qt, err := newQueue(ctx, *projectID, *tempTopic)
	failOnErr(err)

	vf, err := newQueue(ctx, *projectID, *vibeTopic)
	failOnErr(err)

	for {
		//temp
		pub(ctx, qt, makeEvent(*deviceID, "temperature", 15.0, 39.9))

		// vibe
		pub(ctx, vf, makeEvent(*deviceID, "vibration", 0.001, 19.99))

		time.Sleep(freq)
	}

}

func pub(ctx context.Context, q *queue, d []byte) {
	if err := q.push(ctx, d); err != nil {
		logger.Printf("Error on push to %+v: %v", q, err)
	}
}

func failOnErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
