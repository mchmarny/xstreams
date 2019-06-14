all: mod

run:
	go run *.go --freq 2s

mod:
	go mod tidy
	go mod vendor

image: mod
	gcloud builds submit \
		--project cloudylabs-public \
		--tag gcr.io/cloudylabs-public/xstreams:0.1.1

vm:
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

	gcloud compute scp ${GOOGLE_APPLICATION_CREDENTIALS} xstreams:/tmp/sa.pem

	INSTANCE_ID=$(gcloud compute instances describe xstreams --zone us-central1-c --format="value(id)")

	gcloud logging read "resource.type=gce_instance AND \
    	logName=projects/cloudylabs/logs/cos_containers AND \
    	resource.labels.instance_id=${INSTANCE_ID}"

	# gcloud compute ssh eventmaker --zone us-central1-c
	# docker ps
	# docker attach **eventmaker**

vmless:
	gcloud compute instances delete xstreams --zone=us-central1-c

catalog:
	gcloud beta data-catalog entries update \
		--lookup-entry='pubsub.topic.cloudylabs.eventmakertemp' \
		--schema-from-file=schema.yaml

	gcloud beta data-catalog entries update \
		--lookup-entry='pubsub.topic.cloudylabs.eventmakervibe' \
		--schema-from-file=schema.yaml

	gcloud beta data-catalog entries lookup 'pubsub.topic.cloudylabs.eventmakertemp'
	gcloud beta data-catalog entries lookup 'pubsub.topic.cloudylabs.eventmakervibe'
