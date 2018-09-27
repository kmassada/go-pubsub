# go-pubsub

### Create service account

```shell
export APPLICATION=go-pubsub
export APP_SA_NAME=gke-$APPLICATION-sa
gcloud iam service-accounts create $APP_SA_NAME --display-name "GKE $APPLICATION Application Service Account"
export APP_SA_EMAIL=`gcloud iam service-accounts list --format='value(email)' --filter='displayName:$APPLICATION Application Service Account'`
```

### Bind service account policy

```shell
export PROJECT_ID=`gcloud config get-value project`

gcloud projects add-iam-policy-binding $PROJECT_ID \
    --member=serviceAccount:${APP_SA_EMAIL} \
    --role=roles/pubsub.admin
```

### Create service account key and activate it

```shell
gcloud iam service-accounts keys create \
    /home/$USER/$APPLICATION-sa-key.json \
    --iam-account $APP_SA_EMAIL
```

## Application Bootstrap

In this section I create configmaps from the variables we've been gathering to start our application

### Create configmap

```shell
kubectl create configmap project-id --from-literal "project-id=${PROJECT_ID}"
kubectl create configmap $APPLICATION-sa --from-literal "sa-email=${APP_SA_EMAIL}"
kubectl create secret generic $APPLICATION --from-file /home/$USER/$APPLICATION-sa-key.json
```

TODO: Replace this by helm

```shell
CONTAINER_IMAGE=<image path>
```

```shell
envsubst < deployment.template.yaml > deployment.yaml
```

Apply config

```shell
kubectl apply -f deployment.yaml
```