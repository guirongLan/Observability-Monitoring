### Deployare questo funzione in google cloud function
```
gcloud functions deploy alert-handler \
  --runtime go124 \
  --trigger-http \
  --entry-point AlertHandler \
  --allow-unauthenticated \
  --set-env-vars GCP_PROJECT=organic-cat-465614-m9,PUBSUB_TOPIC=alert-topic \
  --region europe-west1
```