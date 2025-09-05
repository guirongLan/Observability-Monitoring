### Deployare questo funzione in google cloud function
```
gcloud functions deploy alert-subscriber \
  --runtime go124 \
  --trigger-topic alert-topic \
  --entry-point AlertSubscriber \
  --set-env-vars "GMAIL_USER=guironglan.cs@gmail.com,GMAIL_APP_PASSWORD=lhqklraxagoncsfc,ALERT_EMAIL=guirong.lan@barsanti.edu.it" \
  --region europe-west1
```