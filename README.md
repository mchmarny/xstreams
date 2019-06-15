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

One of the most common operations in unbounded event stream processing is grouping events by slicing them into period window based on the timestamp of each event. The simplest form of windowing is tumbling which uses consistent duration to group event into non-overlapping time interval. Since our synthetic events have only a one event time, the `tumbling window` is a natural fit.

> You can read more about Windowing [here](https://cloud.google.com/dataflow/docs/guides/sql/streaming-pipeline-basics)

In the Alpha release of Cloud Dataflow SQL in BigQuery joining of multiple unbounded event streams doesn't seem to be yet supported so we are going to create two separate jobs for `temperature` and `vibration`

### Temperature

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

### Vibration

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

You can obviously customize this or write your own, more interesting, SQL queries.

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


## Disclaimer

This is my personal project and it does not represent my employer. I take no responsibility for issues caused by this code. I do my best to ensure that everything works, but if something goes wrong, my apologies is all you will get.

