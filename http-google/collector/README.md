### costruire collector personalizzato
```
docker build -t otel/opentelemetry-collector-contrib:0.128.0
```

### Deployare questo collector artifact registry
```
docker tag otel/opentelemetry-collector-contrib:0.128.0 \
  europe-west8-docker.pkg.dev/organic-cat-465614-m9/http-repository/otel-collector:v0.128.0

docker push europe-west8-docker.pkg.dev/organic-cat-465614-m9/http-repository/otel-collector:v0.128.0
```
### Seleziona l’immagine dal Google Artifact Registry, scegli l’ambiente di deploy (ad esempio Google Cloud Run) e il processo è completato.