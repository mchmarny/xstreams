# xstreams

Stream Joins using Cloud PubSub and SQL over Cloud Dataflow engine in BigQuery

## Run Locally

To start the `eventmaker` and have it mock Cloud PubSub events run the following command on the [released version](https://github.com/mchmarny/pubsub-event-maker/releases) of this binary

```shell
./eventmaker
```

The `eventmaker` also supports the following optional runtime configuration parameters:

* **project** - GCP Project ID (uses either GCP_PROJECT env var or metadata service on GCP)
* **topic** Name of the PubSub topic (will be created if does not exist) [default "eventmaker"]
* **freq** Frequency in which the mocked events will be posted to PubSub topic [default "5s"]
* **sources** Number of event sources to mock [default 1]
* **metric** Name of the metric label for each event [default "utilization"]
* **range** Range of the random metric value [default "0-100"]
* **maxErrors** Maximum number of PubSub push errors [default 10]

So, for example to mock session data tracking user clicks from 10 devices you command would look somehting like this:

```shell
./eventmaker --topic=sessions --sources=10 --metric=clicks --freq=1s
```

Whichever way you configure your `eventmaker` command, The output will look something like this:

```shell
[EVENT-MAKER] Publishing: {"source_id":"device-0","event_id":"eid-b6569857-232c-4e6f-bd51-cda4e81f3e1f","event_ts":"2019-06-05T11:39:50.403778Z","label":"utilization","mem_used":34.47265625,"cpu_used":6.5,"load_1":1.55,"load_5":2.25,"load_15":2.49,"random_metric":94.05090880450125}
```

The JSON payload that will be posted to PubSub topic will have following format:

```json
{
    "source_id": "device-0",
    "event_id": "eid-b6569857-232c-4e6f-bd51-cda4e81f3e1f",
    "event_ts": "2019-06-05T11:39:50.403778Z",
    "label": "utilization",
    "mem_used": 34.47265625,
    "cpu_used": 6.5,
    "load_1": 1.55,
    "load_5": 2.25,
    "load_15": 2.49,
    "random_metric": 94.05090880450125
}
```

## Run in GCE

If you need a sustained flow of events you can use the `glcoud` CLI to spin up a small VM with the `eventmaker` container.

```shell
gcloud compute instances create-with-container eventmaker \
    --container-image gcr.io/cloudylabs-public/pubsub-event-maker:0.1.5 \
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

This will deploy the image into a VM on GCE. To complete the install you will have to also uppload a GCP service account file so the container can start.

```shell
gcloud compute scp ${GOOGLE_APPLICATION_CREDENTIALS} eventmaker:/tmp/sa.pem
```

> Chance are your default GCP credentials are a lot more powerful than the `eventmaker` needs. To follow the least privilege principle, you can create a brand new service account that has only the right required for PubSub (`projects.topics.create` and `projects.topics.publish`)

If you want to monitor the logs output from container you will need to capture the instance ID first

```shell
INSTANCE_ID=$(gcloud compute instances describe eventmaker --zone us-central1-c --format="value(id)")
```

Then you can output the logs using this command

```shell
gcloud logging read "resource.type=gce_instance \
    AND logName=projects/cloudylabs/logs/cos_containers \
    AND resource.labels.instance_id=${INSTANCE_ID}"
```

## Cost

There is a pretty generous free tier (first 10GB) on PubSub which is priced based on data volume transmitted in a calendar month. For more information see [PubSub Pricing](https://cloud.google.com/pubsub/pricing)


## Disclaimer

This is my personal project and it does not represent my employer. I take no responsibility for issues caused by this code. I do my best to ensure that everything works, but if something goes wrong, my apologies is all you will get.