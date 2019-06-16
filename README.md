# xstreams

How to joins multiple event streams using Cloud PubSub and Cloud Dataflow SQL in BigQuery.

This demo will cover:

* [Generate Events](#generate-events) how to generator synthetic temperature (Celsius) and vibration (mm/s2) metrics and publish them onto Cloud PubSub topic
* [Process Events](#process-events) how to process the two event streams and use Cloud Dataflow windowing function to join the two unbounded event streams
* [Analyze Data](#analyze-data) how to use BigQuery SQL to analyze the resulting event data

## Generate Events

The included `xstreams` data generation utility publishes mocked up events onto PubSub topic in form of a JSON payload for both temperature and friction:

```json
{
    "source_id": "device1",
    "event_id": "eid-243303fe-bc7c-4f3e-b9fd-7dc1929f9ae9",
    "event_time": 1560484054,
    "metric": "temperature",
    "value": 17.48582634964145
}
```

While you can run the `xstreams` data generation utility locally, the simplest way to generate sustained stream of events is by using Cloud compute resources. To do that we will use the `glcoud` CLI to spin up a small VM with the `xstreams` container.

> For more information how to install and configure `glcoud` see [here](https://cloud.google.com/sdk/install)


```shell
gcloud compute instances create-with-container xstreams \
    --container-image=gcr.io/cloudylabs-public/xstreams:0.1.1 \
    --zone=us-central1-c \
    --image-family=cos-stable \
    --image-project=cos-cloud \
    --scopes=cloud-platform \
    --container-env="GOOGLE_APPLICATION_CREDENTIALS=/tmp/xstreams.pem" \
    --container-mount-host-path=mount-path=/tmp,host-path=/tmp,mode=rw
```

This command will deploy the prebuilt image into a VM on GCE. To complete the install you will have to also upload also a GCP service account file so that the container can start.

```shell
gcloud compute scp $GCP_KEYS/xstreams.pem xstreams:/tmp/xstreams.pem
```

> Your default GCP credentials can do a lot more than `xstreams` needs. To follow the least privilege principle, you should create a brand new service account that has only the necessary `Pub/Sub Editor` role. For more information on how to crate a service account and generate its keys see [here](https://cloud.google.com/iam/docs/creating-managing-service-accounts)

To monitor logs output from `xstreams` container you will need to capture the instance ID first

```shell
INSTANCE_ID=$(gcloud compute instances describe xstreams --zone us-central1-c --format="value(id)")
```

Then you can output the logs using this command:

```shell
gcloud logging read "resource.type=gce_instance \
    AND logName=projects/cloudylabs/logs/cos_containers \
    AND resource.labels.instance_id=${INSTANCE_ID}"
```


## Process Events


### Raw Events

First we are going to load the raw event data from the two PubSub topics into single BigQuery table. Let's create the BigQuery dataset and table:

```shell
bq mk xstreams
bq query --use_legacy_sql=false "
CREATE OR REPLACE TABLE xstreams.raw_events (
  source_id STRING NOT NULL,
  event_id STRING NOT NULL,
  event_time INT64 NOT NULL,
  metric STRING NOT NULL,
  value FLOAT64 NOT NULL
)"
```

Once the table is created, we can create Dataflow job to drain the payloads from the `eventmakertemp` and `eventmakervibe` topics to `raw_events` table in BigQuery


```shell
PROJECT=$(gcloud config get-value project)

gcloud dataflow jobs run xstreams-raw-tepm-events \
  --gcs-location gs://dataflow-templates/latest/PubSub_to_BigQuery \
  --parameters "inputTopic=projects/${PROJECT}/topics/eventmakertemp,outputTableSpec=${PROJECT}:xstreams.raw_events"

gcloud dataflow jobs run xstreams-raw-vibe-events \
  --gcs-location gs://dataflow-templates/latest/PubSub_to_BigQuery \
  --parameters "inputTopic=projects/${PROJECT}/topics/eventmakervibe,outputTableSpec=${PROJECT}:xstreams.raw_events"
```

Dataflow will take a couple of minutes to create the necessary resources. When done, you will see data in the `kadvice.raw_events` table in BigQuery.

### Windowing

One of the most common operations in unbounded event stream processing is grouping events by slicing them into period window based on the timestamp of each event. The simplest form of windowing is tumbling which uses consistent duration to group events into non-overlapping time interval. Since our synthetic events have only a one event time, the `tumbling window` is a natural fit.

> You can read more about Windowing [here](https://cloud.google.com/dataflow/docs/guides/sql/streaming-pipeline-basics)

In the Alpha release of Cloud Dataflow SQL in BigQuery joining of multiple unbounded event streams doesn't seem to be yet supported. So, we are going to create two separate jobs: `temperature` and `vibration`.

#### Temperature

Execute the following query with the Dataflow engine and save the results to `xstreams.temp_tumble_30`.

Notice the use of `TUMBLE_START` function which will return the starting timestamp of each one of our events. The use of event payload timestamps is not supported yet, so for now, we are going to use the time when this event was published onto PubSub topic (`event_timestamp` of the PubSub wrapper).

```sql
SELECT
   t.payload.source_id,
   TUMBLE_START("INTERVAL 30 SECOND") AS period_start,

   MIN(t.payload.value) as min_temp_period_val,
   AVG(t.payload.value) as avg_temp_period_val,
   MAX(t.payload.value) as max_temp_period_val,
   MIN(t.payload.event_time) as min_temp_event_time,
   MAX(t.payload.event_time) as max_temp_event_time

 FROM pubsub.topic.cloudylabs.eventmakertemp t
 GROUP BY
   t.payload.source_id,
   TUMBLE(t.event_timestamp, "INTERVAL 30 SECOND")
```

#### Vibration

Similarly execute the following with the Dataflow engine and save the results to `xstreams.vibe_tumble_30`.

```sql
SELECT
   v.payload.source_id,
   TUMBLE_START("INTERVAL 30 SECOND") AS period_start,

   MIN(v.payload.value) as min_vibe_period_val,
   AVG(v.payload.value) as avg_vibe_period_val,
   MAX(v.payload.value) as max_vibe_period_val,
   MIN(v.payload.event_time) as min_vibe_event_time,
   MAX(v.payload.event_time) as max_vibe_event_time

 FROM pubsub.topic.cloudylabs.eventmakervibe v
 GROUP BY
   v.payload.source_id,
   TUMBLE(v.event_timestamp, "INTERVAL 30 SECOND")
```

## Analyze Data

Now that we have both `temp_tumble_30` and `vibe_tumble_30` tables created, we can switch back to BigQuery query engine to analyze the data using SQL.

Here is an "uber query" example that joins temperature and vibrations using `source_id` and `period_start` to:

* find `min`, `avg`, and `max` values for each event in each `30 sec` period
* compare `period_start` to the event time to derive `min` and `max` delta
* find `min`, `max` event time within each processing period

```sql
SELECT
  t.period_start,
  t.min_temp_period_val,
  t.avg_temp_period_val,
  t.max_temp_period_val,
  TIMESTAMP_SECONDS(t.min_temp_event_time) as min_temp_event_time,
  TIMESTAMP_SECONDS(t.max_temp_event_time) AS max_temp_event_time,
  TIMESTAMP_DIFF(TIMESTAMP_SECONDS(t.min_temp_event_time), t.period_start, SECOND) as first_temp_period_event_time,
  TIMESTAMP_DIFF(TIMESTAMP_SECONDS(t.max_temp_event_time), t.period_start, SECOND) as last_temp_period_event_time,
  v.min_vibe_period_val,
  v.avg_vibe_period_val,
  v.max_vibe_period_val,
  TIMESTAMP_SECONDS(v.min_vibe_event_time) as min_vibe_event_time,
  TIMESTAMP_SECONDS(v.max_vibe_event_time) as max_vibe_event_time,
  TIMESTAMP_DIFF(TIMESTAMP_SECONDS(v.min_vibe_event_time), t.period_start, SECOND) as first_vibe_period_event_time,
  TIMESTAMP_DIFF(TIMESTAMP_SECONDS(v.max_vibe_event_time), t.period_start, SECOND) as last_vibe_period_event_time
FROM xstreams.vibe_tumble_30 v
INNER JOIN xstreams.temp_tumble_30 t ON
  t.source_id = v.source_id AND
  t.period_start = v.period_start
ORDER BY v.period_start DESC
```

Similarly, to identify temperatures in current period that are above the 10 minute average:

```sql
SELECT
  TIMESTAMP_SECONDS(event_time) as event_time,
  source_id,
  value
FROM xstreams.raw_events
WHERE
  metric = 'temperature'
  AND TIMESTAMP_SECONDS(event_time) > (
    SELECT MAX(period_start)
    FROM xstreams.temp_tumble_30
  )
  AND value > (
    SELECT AVG(avg_temp_period_val)
    FROM xstreams.temp_tumble_30
    WHERE period_start > TIMESTAMP_SUB(CURRENT_TIMESTAMP(), INTERVAL 10 MINUTE)
  )
```

And finally to identify outliers for either temperature of vibration for a given period within last 5 minutes you can run this query:

```sql
SELECT
  TIMESTAMP_SECONDS(e.event_time) as event_time,
  e.source_id,
  e.metric,
  e.value,
  CASE
    WHEN e.metric = 'temperature' THEN AVG(t.avg_temp_period_val)
    WHEN e.metric = 'vibration' THEN AVG(v.avg_vibe_period_val)
    ELSE 0
  END as avg_period_value,
  CASE
    WHEN e.metric = 'temperature' THEN e.value - AVG(t.avg_temp_period_val)
    WHEN e.metric = 'vibration' THEN e.value - AVG(v.avg_vibe_period_val)
    ELSE 0
  END as avg_period_delta
FROM xstreams.raw_events e
INNER JOIN xstreams.temp_tumble_30 t ON e.source_id = t.source_id AND e.event_time BETWEEN t.min_temp_event_time AND t.max_temp_event_time
INNER JOIN xstreams.vibe_tumble_30 v ON e.source_id = v.source_id AND e.event_time BETWEEN v.min_vibe_event_time AND v.max_vibe_event_time
WHERE
  TIMESTAMP_SECONDS(e.event_time) > TIMESTAMP_SUB(CURRENT_TIMESTAMP(), INTERVAL 5 MINUTE)
GROUP BY
  e.event_time,
  e.source_id,
  e.metric,
  e.value
ORDER BY 1 DESC
```

You can obviously customize each one of these or write your own, more interesting, SQL queries.

## Cost

There is a pretty generous [free tier](https://cloud.google.com/free/) on GCP

* PubSub - is priced based on data volume transmitted in a calendar month (first 10GB free). For more information see [PubSub Pricing](https://cloud.google.com/pubsub/pricing)
* GCE - the `n1-standard-1` VM used in this example is $0.0475/hr. For more information see [Compute Pricing](https://cloud.google.com/compute/pricing)
* Dataflow an BigQuery - pricing for Dataflow an BigQuery is more complicated as it is based on usage of vCPU, RAM, Storage, and data processing per GB. For details see [Cloud Dataflow pricing](https://cloud.google.com/dataflow/pricing) and [BigQuery pricing](https://cloud.google.com/bigquery/pricing)

## Cleanup

To delete the data generation VM

```shell
gcloud compute instances delete xstreams --zone=us-central1-c
```

To delete the two topics created by `xstreams`

```shell
gcloud pubsub topics delete eventmakertemp
gcloud pubsub topics delete eventmakervibe
```

To delete BigQuery dataset created by `xstreams`


```shell
bq rm -r -f xstreams
```

To stop all the Dataflow jobs created in this demo (unless you named them explicitly), you will have to go to the Google Cloud Console and delete them manually


## Disclaimer

This is my personal project and it does not represent my employer. I take no responsibility for issues caused by this code. I do my best to ensure that everything works, but if something goes wrong, my apologies is all you will get.

