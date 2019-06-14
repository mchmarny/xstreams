# xstreams

Stream Joins using Cloud PubSub and SQL over Cloud Dataflow engine in BigQuery

//TODO: Describe what this shows, and breakout on run and analyze

## Generate Events

### Run Locally

> Note, if you don't have `go` installed, you can use the [Run in GCE](#run-in-gce) section below

To run `xstreams` locally execute:

```shell
go run *.go --freq 2s
```

The `xstreams` also supports the following optional runtime configuration parameters:

* **project** GCP Project ID (uses either GCP_PROJECT env var or metadata service on GCP)
* **device** Name of the device to send metrics from [device1]
* **temp-topic** PubSub topic for temperatures (will be created if does not exist) [eventmakertemp]
* **vibe-topic** PubSub topic for vibrations (will be created if does not exist) [eventmakervibe]
* **freq** Frequency in which the mocked events will be posted to PubSub topic [default "5s"]

`xstreams` will output something like this:

```shell
[EM] vibration[device1]: {"source_id":"device1","event_id":"eid-327da1d3-559f-4897-b90e-3d979dcb2d24","event_time":1560484052,"metric":"vibration","value":8.82181907995207}
[EM] temperature[device1]: {"source_id":"device1","event_id":"eid-243303fe-bc7c-4f3e-b9fd-7dc1929f9ae9","event_time":1560484054,"metric":"temperature","value":17.48582634964145}
```

The JSON payload published to each one of the topics will look like this:

```json
{
    "source_id": "device1",
    "event_id": "eid-243303fe-bc7c-4f3e-b9fd-7dc1929f9ae9",
    "event_time": 1560484054,
    "metric": "temperature",
    "value": 17.48582634964145
}
```

### Run in GCE

If you need a sustained flow of events you can use the `glcoud` CLI to spin up a small VM with the `xstreams` container.

```shell
gcloud compute instances create-with-container xstreams \
    --container-image gcr.io/cloudylabs-public/xstreams:0.1.1 \
    --machine-type n1-standard-1 \
    --zone us-central1-c \
    --image-family=cos-stable \
    --image-project=cos-cloud \
    --maintenance-policy MIGRATE \
    --container-restart-policy=always \
    --scopes "cloud-platform" \
    --container-privileged \
    --container-env="GOOGLE_APPLICATION_CREDENTIALS=/tmp/sa.pem" \
    --container-mount-host-path=mount-path=/tmp,host-path=/tmp,mode=rw
```

This command will deploy the prebuild image into a VM on GCE. To complete the install you will have to also uppload a GCP service account file so the container can start.

```shell
gcloud compute scp ${GOOGLE_APPLICATION_CREDENTIALS} xstreams:/tmp/sa.pem
```

> Your default GCP credentials pretty powerful, a lot more so than the `xstreams` needs. To follow the least privilege principle, you can create a brand new service account that has only the rights required for PubSub (`projects.topics.create` and `projects.topics.publish`)

If you want to monitor the logs output from container you will need to capture the instance ID first

```shell
INSTANCE_ID=$(gcloud compute instances describe xstreams --zone us-central1-c --format="value(id)")
```

Then you can output the logs using this command

```shell
gcloud logging read "resource.type=gce_instance \
    AND logName=projects/cloudylabs/logs/cos_containers \
    AND resource.labels.instance_id=${INSTANCE_ID}"
```

## Tumble WIndows

### Temperature

> Execute with the Dataflow engine and save the results to `xstreams.temp_tumble_30`

```sql
SELECT
   t.payload.source_id,
   TUMBLE_START("INTERVAL 30 SECOND") AS period_start,

   MIN(t.payload.value) as min_temperature,
   AVG(t.payload.value) as avg_temperature,
   MAX(t.payload.value) as max_temperature

 FROM pubsub.topic.cloudylabs.eventmakertemp t
 GROUP BY
   t.payload.source_id,
   TUMBLE(t.event_timestamp, "INTERVAL 30 SECOND")
```

### Vibration

> Execute with the Dataflow engine and save the results to `xstreams.vibe_tumble_30`

```sql
SELECT
   v.payload.source_id,
   TUMBLE_START("INTERVAL 30 SECOND") AS period_start,

   MIN(v.payload.value) as min_vibration,
   AVG(v.payload.value) as avg_vibration,
   MAX(v.payload.value) as max_vibration

 FROM pubsub.topic.cloudylabs.eventmakervibe v
 GROUP BY
   v.payload.source_id,
   TUMBLE(v.event_timestamp, "INTERVAL 30 SECOND")
```

## Analyze Data

> Execute these with the BigQuery engine

```sql
SELECT
  v.period_start,
  v.min_vibration,
  v.avg_vibration,
  v.max_vibration,
  t.min_temperature,
  t.avg_temperature,
  t.max_temperature
FROM xstreams.vibe_tumble_30 v
JOIN xstreams.temp_tumble_30 t ON
  t.source_id = v.source_id AND
  t.period_start = v.period_start
ORDER BY v.period_start DESC
```


## Cost

There is a pretty generous free tier on GCP

* PubSub - (first 10GB) which is priced based on data volume transmitted in a calendar month. For more information see [PubSub Pricing](https://cloud.google.com/pubsub/pricing)
* GCE -
* Dataflow -
* BigQuery -

## Disclaimer

This is my personal project and it does not represent my employer. I take no responsibility for issues caused by this code. I do my best to ensure that everything works, but if something goes wrong, my apologies is all you will get.

